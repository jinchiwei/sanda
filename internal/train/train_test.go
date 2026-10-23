package train

import (
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/jinchiwei/sanda/internal/analysis"
	"github.com/jinchiwei/sanda/internal/store"
)

func TestDrillContainsFocusUnits(t *testing.T) {
	cases := []string{"ck", "e,", "e ", " q", "nk"} // capitals: TestCapitalDrillIsSentences
	for _, u := range cases {
		words := Drill([]Focus{{Unit: u}}, 40, 1)
		text := " " + strings.Join(words, " ") + " "
		if c := strings.Count(text, u); c < 15 {
			t.Errorf("unit %q appears %d times in drill: %s", u, c, text)
		}
	}
}

func TestPaceTarget(t *testing.T) {
	tests := []store.Test{{Kind: store.KindOfficial, WPM: 100}, {Kind: store.KindTrain, WPM: 300}, {Kind: store.KindOfficial, WPM: 120}}
	if got := PaceTarget(tests, 1.1); got < 120.9 || got > 121.1 {
		t.Fatalf("target = %v, want 121 (median 110 × 1.1, train rounds ignored)", got)
	}
	if PaceTarget(nil, 1.07) != 60 {
		t.Fatal("default target should be 60")
	}
}

func model(units ...*analysis.UnitStat) analysis.Model {
	m := analysis.Model{Units: map[string]*analysis.UnitStat{}}
	for _, u := range units {
		m.Units[u.Unit] = u
		m.Ranked = append(m.Ranked, u)
	}
	return m
}

// Capitals slow a whole round, so they're drilled on their own and never mixed into lowercase rounds.
func TestCapitalsGetTheirOwnRound(t *testing.T) {
	m := model(
		&analysis.UnitStat{Unit: " I", Class: analysis.ClassShift, Cost: 3},
		&analysis.UnitStat{Unit: " o", Class: analysis.ClassWordStart, Cost: 2},
		&analysis.UnitStat{Unit: "ll", Class: analysis.ClassLetters, Cost: 1},
	)
	now := time.Now()
	f := PickFocus(m, nil, now, 3, Prose)
	if len(f) != 1 || f[0].Unit != " I" {
		t.Fatalf("top weak spot is a capital: want a round of just \" I\", got %+v", f)
	}
	sched := map[string]store.Sched{Prose.SchedKey(" I"): {Due: now.Add(Retry)}}
	for _, x := range PickFocus(m, sched, now, 3, Prose) {
		if x.Unit == " I" {
			t.Fatalf("resting capital came back: %+v", x)
		}
	}
	m = model(
		&analysis.UnitStat{Unit: " o", Class: analysis.ClassWordStart, Cost: 3},
		&analysis.UnitStat{Unit: " I", Class: analysis.ClassShift, Cost: 2},
		&analysis.UnitStat{Unit: "ll", Class: analysis.ClassLetters, Cost: 1},
	)
	f = PickFocus(m, nil, now, 3, Prose)
	if len(f) != 2 || f[0].Unit != " o" || f[1].Unit != "ll" {
		t.Fatalf("lowercase round must leave the capital out, got %+v", f)
	}
}

func TestCapitalDrillIsSentences(t *testing.T) {
	words := Drill([]Focus{{Unit: " I"}}, 40, 3)
	caps := 0
	for i, w := range words {
		if unicode.IsUpper([]rune(w)[0]) {
			caps++
			if i > 0 && !strings.HasSuffix(words[i-1], ".") {
				t.Fatalf("capital %q follows %q, not a sentence end: %v", w, words[i-1], words)
			}
		}
	}
	if caps < 10 || caps > 20 {
		t.Fatalf("%d capitals in 40 words; want one per short sentence: %v", caps, words)
	}
}

// Time/words training sticks to what those tests contain, and learns only from them.
func TestPlainFormat(t *testing.T) {
	m := model(
		&analysis.UnitStat{Unit: " I", Class: analysis.ClassShift, Cost: 4},
		&analysis.UnitStat{Unit: "e,", Class: analysis.ClassPunct, Cost: 3},
		&analysis.UnitStat{Unit: " o", Class: analysis.ClassWordStart, Cost: 2},
		&analysis.UnitStat{Unit: "ll", Class: analysis.ClassLetters, Cost: 1},
	)
	f := PickFocus(m, nil, time.Now(), 3, Plain)
	if len(f) != 2 || f[0].Unit != " o" || f[1].Unit != "ll" {
		t.Fatalf("plain round should drill only lowercase units, got %+v", f)
	}
	for _, w := range Drill(f, 60, 5) {
		for _, r := range w {
			if !unicode.IsLower(r) {
				t.Fatalf("plain drill word %q has %q", w, r)
			}
		}
	}
	cases := []struct {
		t     store.Test
		plain bool
	}{
		{store.Test{Kind: store.KindOfficial, Mode: "time"}, true},
		{store.Test{Kind: store.KindOfficial, Mode: "words"}, true},
		{store.Test{Kind: store.KindOfficial, Mode: "words", Punctuation: true}, false},
		{store.Test{Kind: store.KindOfficial, Mode: "quote"}, false},
		{store.Test{Kind: store.KindTrain, Mode: "drill-plain"}, true},
		{store.Test{Kind: store.KindTrain, Mode: "drill-prose"}, false},
	}
	if Plain.Counts(store.Test{Kind: store.KindTrain, Mode: "drill"}) || Prose.Counts(store.Test{Kind: store.KindTrain, Mode: "drill"}) {
		t.Error("old mixed drills belong to neither format")
	}
	for _, c := range cases {
		if Plain.Counts(c.t) != c.plain || Prose.Counts(c.t) == c.plain {
			t.Errorf("%+v: plain=%v prose=%v, want plain=%v", c.t, Plain.Counts(c.t), Prose.Counts(c.t), c.plain)
		}
	}
	if Plain.Counts(store.Test{Kind: store.KindPace}) || Prose.Counts(store.Test{Kind: store.KindPace}) {
		t.Error("pace rounds belong to neither")
	}
}
