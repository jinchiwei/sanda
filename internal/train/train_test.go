package train

import (
	"strings"
	"testing"

	"github.com/jinchiwei/sanda/internal/store"
)

func TestDrillContainsFocusUnits(t *testing.T) {
	cases := []string{"ck", " T", "e,", "e ", " q", "nk"}
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
