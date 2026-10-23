package analysis

import (
	"strings"
	"testing"
	"time"

	"github.com/jinchiwei/sanda/internal/engine"
	"github.com/jinchiwei/sanda/internal/store"
)

// typeText types text with a per-transition delay chosen by slow(from, to).
func typeText(text string, slow func(a, b rune) time.Duration) []engine.Event {
	words := strings.Fields(text)
	s := engine.New(words)
	t := time.Unix(0, 0)
	prev := rune(0)
	for _, r := range text {
		t = t.Add(slow(prev, r))
		if r == ' ' {
			s.Space(t)
		} else {
			s.Type(r, t)
		}
		prev = r
	}
	return s.Events
}

func TestTransitionsAndClasses(t *testing.T) {
	ev := typeText("The cat sat.", func(a, b rune) time.Duration { return 100 * time.Millisecond })
	clean, misses := Transitions(ev)
	if len(misses) != 0 || len(clean) != len("The cat sat.")-1 {
		t.Fatalf("clean=%d misses=%d", len(clean), len(misses))
	}
	if clean[0].Unit() != "Th" || Classify(' ', 'T') != ClassShift || Classify('t', '.') != ClassPunct ||
		Classify(' ', 'c') != ClassWordStart || Classify('e', ' ') != ClassSpace {
		t.Fatal("classification wrong")
	}
}

func TestMissRecorded(t *testing.T) {
	s := engine.New([]string{"ab"})
	t0 := time.Unix(0, 0)
	s.Type('a', t0)
	s.Type('x', t0.Add(time.Second/10)) // wrong on a→b
	_, misses := Transitions(s.Events)
	if len(misses) != 1 || misses[0].Unit() != "ab" {
		t.Fatalf("misses = %+v", misses)
	}
}

func TestBreakdownFindsShiftCost(t *testing.T) {
	text := strings.Repeat("The cat and The dog and ", 6)
	ev := typeText(text, func(a, b rune) time.Duration {
		if b == 'T' {
			return 400 * time.Millisecond // slow capitals
		}
		return 80 * time.Millisecond
	})
	losses := Breakdown(ev, strings.Fields(text), 30*time.Second)
	if len(losses) == 0 || !strings.HasPrefix(losses[0].Label, "Shift") {
		t.Fatalf("expected Shift as the top loss, got %+v", losses)
	}
}

func TestModelRanksSlowPair(t *testing.T) {
	text := strings.Repeat("the quick brown fox ", 20)
	ev := typeText(text, func(a, b rune) time.Duration {
		if a == 'c' && b == 'k' {
			return 350 * time.Millisecond
		}
		return 90 * time.Millisecond
	})
	m := BuildModel([]store.TestEvents{{Test: store.Test{StartedAt: time.Now()}, Events: ev}})
	if len(m.Ranked) == 0 || m.Ranked[0].Unit != "ck" {
		got := ""
		if len(m.Ranked) > 0 {
			got = m.Ranked[0].Unit
		}
		t.Fatalf("top weakness = %q, want ck", got)
	}
}

// Word starts are slower than letter pairs for everyone. One at par with the other word starts
// is not a weak spot, however common it is.
func TestWordStartJudgedAgainstWordStarts(t *testing.T) {
	text := strings.Repeat("to the on of to it on at ", 20)
	ev := typeText(text, func(a, b rune) time.Duration {
		if a == ' ' {
			return 150 * time.Millisecond
		}
		return 70 * time.Millisecond
	})
	m := BuildModel([]store.TestEvents{{Test: store.Test{StartedAt: time.Now()}, Events: ev}})
	for _, u := range m.Ranked {
		if u.Class == ClassWordStart {
			t.Errorf("%q ranked weak at %v, par %v: every word start is that speed", u.Unit, u.Median, u.Target)
		}
	}
}

// A capital is held to its lowercase word start plus a Shift allowance, so the commonest capital
// counts as weak even when it's the fastest capital you have.
func TestCapitalJudgedAgainstLowercase(t *testing.T) {
	text := strings.Repeat("I is in it A an at ", 20)
	ev := typeText(text, func(a, b rune) time.Duration {
		switch {
		case a == ' ' && b == 'I':
			return 160 * time.Millisecond
		case a == ' ' && b == 'A':
			return 250 * time.Millisecond
		case a == ' ':
			return 90 * time.Millisecond
		}
		return 70 * time.Millisecond
	})
	m := BuildModel([]store.TestEvents{{Test: store.Test{StartedAt: time.Now()}, Events: ev}})
	u := m.Units[" I"]
	if u == nil || u.Cost <= 0 {
		t.Fatalf(`" I" at 160ms vs lowercase word starts at 90ms should be weak, got %+v`, u)
	}
	if want := 90*time.Millisecond + 36*time.Millisecond; u.Target != want {
		t.Errorf(`" I" par = %v, want %v (lowercase par + 40%%)`, u.Target, want)
	}
}
