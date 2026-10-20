package engine

import (
	"math"
	"testing"
	"time"
)

// typeAll types s one keystroke every step, starting at t0.
func typeAll(s *Session, text string, t0 time.Time, step time.Duration) time.Time {
	t := t0
	for _, r := range text {
		if r == ' ' {
			s.Space(t)
		} else {
			s.Type(r, t)
		}
		t = t.Add(step)
	}
	return t
}

func TestPerfectWordsSession(t *testing.T) {
	s := New([]string{"hello", "world"})
	t0 := time.Unix(0, 0)
	// 11 keystrokes, 100 ms apart: the last lands at 1.0 s and completes the test.
	typeAll(s, "hello world", t0, 100*time.Millisecond)
	if !s.Finished() {
		t.Fatal("session should finish when the last word is complete")
	}
	st := s.Stats()
	if st.Duration != time.Second {
		t.Fatalf("duration = %v, want 1s", st.Duration)
	}
	// 10 letters + 1 space = 11 chars over 1 s → 11/5*60 = 132 WPM.
	if math.Abs(st.WPM-132) > 0.01 || st.Acc != 100 || st.Correct != 10 || st.Missed != 0 {
		t.Fatalf("stats = %+v", st)
	}
}

func TestErrorsExtraAndMissed(t *testing.T) {
	s := New([]string{"cat", "dog"})
	t0 := time.Unix(0, 0)
	end := typeAll(s, "cxtt do ", t0, 100*time.Millisecond)
	_ = end
	st := s.Stats()
	// "cxtt" vs "cat": c ok, x wrong, t ok, extra t. "do" vs "dog": 1 missed.
	if st.Correct != 4 || st.Incorrect != 1 || st.Extra != 1 || st.Missed != 1 {
		t.Fatalf("chars = %d/%d/%d/%d", st.Correct, st.Incorrect, st.Extra, st.Missed)
	}
	if st.WPM != 0 {
		t.Fatalf("no correct words, WPM = %v", st.WPM)
	}
}

func TestBackspaceIntoWrongWordOnly(t *testing.T) {
	s := New([]string{"ab", "cd", "ef"})
	t0 := time.Unix(0, 0)
	typeAll(s, "ab ", t0, time.Millisecond)
	s.Backspace(t0.Add(time.Second))
	if s.Cur != 1 {
		t.Fatal("must not step back into a correct word")
	}
	typeAll(s, "cx ", t0.Add(2*time.Second), time.Millisecond)
	s.Backspace(t0.Add(3 * time.Second))
	if s.Cur != 1 || string(s.Input[1]) != "cx" {
		t.Fatalf("should step back into the wrong word, cur=%d input=%q", s.Cur, string(s.Input[1]))
	}
}

func TestFirstTryFlag(t *testing.T) {
	s := New([]string{"ab", "c"})
	t0 := time.Unix(0, 0)
	s.Type('x', t0)
	s.Backspace(t0.Add(time.Millisecond))
	s.Type('a', t0.Add(2*time.Millisecond))
	if s.Events[0].First != true || s.Events[2].First != false {
		t.Fatal("retyping a position must not count as first try")
	}
}

func TestTimedSessionCountsPartialWord(t *testing.T) {
	s := New([]string{"aaaa", "bbbb"})
	s.TimeLimit = time.Second
	s.More = func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = "zz"
		}
		return out
	}
	t0 := time.Unix(0, 0)
	typeAll(s, "aaaa bb", t0, 100*time.Millisecond)
	s.Tick(t0.Add(2 * time.Second))
	st := s.Stats()
	if !s.Finished() || st.Duration != time.Second {
		t.Fatalf("finished=%v dur=%v", s.Finished(), st.Duration)
	}
	// "aaaa " (5) + partial "bb" (2) = 7 chars in 1 s = 84 WPM; partial word is not missed.
	if math.Abs(st.WPM-84) > 0.01 || st.Missed != 0 {
		t.Fatalf("wpm=%v missed=%d", st.WPM, st.Missed)
	}
}

func TestLiveWPM(t *testing.T) {
	s := New([]string{"hello", "world", "again"})
	t0 := time.Unix(0, 0)
	typeAll(s, "hello wor", t0, 100*time.Millisecond)
	// At 1.2 s: "hello " (6) + "wor" (3) = 9 correct chars → 9/5/0.02 min = 90 WPM.
	if got := s.LiveWPM(t0.Add(1200 * time.Millisecond)); math.Abs(got-90) > 0.01 {
		t.Fatalf("live wpm = %v, want 90", got)
	}
	s.Type('x', t0.Add(1300*time.Millisecond)) // a wrong letter drops the partial word
	if got := s.LiveWPM(t0.Add(1500 * time.Millisecond)); math.Abs(got-48) > 0.01 {
		t.Fatalf("live wpm after error = %v, want 48", got)
	}
}
