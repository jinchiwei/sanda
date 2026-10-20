// Package engine runs one typing session: Monkeytype-style word input, a per-keystroke timing log and scoring.
package engine

import (
	"math"
	"time"
)

type Kind int

const (
	KindChar Kind = iota
	KindSpace
	KindBackspace
	KindDeleteWord
)

// Event is one keystroke. T is measured from the first keystroke of the session.
type Event struct {
	T        time.Duration
	Kind     Kind
	Typed    rune
	Expected rune // 0 for extra characters past the end of the word
	Correct  bool
	First    bool // first keystroke ever made at this position
	Word     int
	Pos      int
}

type Session struct {
	Words     [][]rune
	Input     [][]rune
	Cur       int
	TimeLimit time.Duration // 0 = no limit
	// More supplies extra words for unlimited (time) sessions.
	More func(n int) []string

	Events   []Event
	start    time.Time
	started  bool
	finished bool
	lastDone bool // the final word was completed (not cut off by the timer)
	duration time.Duration
	tried    map[[2]int]bool
}

func New(words []string) *Session {
	s := &Session{tried: map[[2]int]bool{}}
	s.add(words)
	s.Input = make([][]rune, 1, len(s.Words))
	return s
}

func (s *Session) add(words []string) {
	for _, w := range words {
		if w != "" {
			s.Words = append(s.Words, []rune(w))
		}
	}
}

func (s *Session) Started() bool  { return s.started }
func (s *Session) Finished() bool { return s.finished }

// Elapsed is the time since the first keystroke (frozen once finished).
func (s *Session) Elapsed(now time.Time) time.Duration {
	if !s.started {
		return 0
	}
	if s.finished {
		return s.duration
	}
	return now.Sub(s.start)
}

func (s *Session) begin(now time.Time) {
	if !s.started {
		s.started = true
		s.start = now
	}
}

func (s *Session) finish(now time.Time) {
	if s.finished {
		return
	}
	s.finished = true
	s.duration = now.Sub(s.start)
	if s.TimeLimit > 0 && s.duration > s.TimeLimit {
		s.duration = s.TimeLimit
	}
}

func (s *Session) log(now time.Time, e Event) {
	e.T = now.Sub(s.start)
	key := [2]int{e.Word, e.Pos}
	if e.Kind == KindChar || e.Kind == KindSpace {
		e.First = !s.tried[key]
		s.tried[key] = true
	}
	s.Events = append(s.Events, e)
}

func (s *Session) ensureWords() {
	if s.More != nil && len(s.Words)-s.Cur < 60 {
		s.add(s.More(100))
	}
}

// Tick ends a timed session once its time is up.
func (s *Session) Tick(now time.Time) {
	if s.started && !s.finished && s.TimeLimit > 0 && now.Sub(s.start) >= s.TimeLimit {
		s.finish(s.start.Add(s.TimeLimit))
	}
}

// Type handles a printable character.
func (s *Session) Type(r rune, now time.Time) {
	if s.finished {
		return
	}
	s.begin(now)
	s.Tick(now)
	if s.finished {
		return
	}
	word, in := s.Words[s.Cur], s.Input[s.Cur]
	pos := len(in)
	if pos >= len(word)+20 { // cap runaway extra characters like Monkeytype
		return
	}
	var exp rune
	if pos < len(word) {
		exp = word[pos]
	}
	s.Input[s.Cur] = append(in, r)
	s.log(now, Event{Kind: KindChar, Typed: r, Expected: exp, Correct: exp != 0 && r == exp, Word: s.Cur, Pos: pos})
	if s.Cur == len(s.Words)-1 && s.More == nil && string(s.Input[s.Cur]) == string(word) {
		s.lastDone = true
		s.finish(now)
	}
}

// Space submits the current word.
func (s *Session) Space(now time.Time) {
	if s.finished || len(s.Input[s.Cur]) == 0 {
		return
	}
	s.begin(now)
	s.Tick(now)
	if s.finished {
		return
	}
	word, in := s.Words[s.Cur], s.Input[s.Cur]
	exp := ' '
	if len(in) < len(word) {
		exp = word[len(in)]
	}
	ok := string(in) == string(word)
	s.log(now, Event{Kind: KindSpace, Typed: ' ', Expected: exp, Correct: ok, Word: s.Cur, Pos: len(word)})
	if s.Cur == len(s.Words)-1 && s.More == nil {
		s.lastDone = true
		s.finish(now)
		return
	}
	s.Cur++
	s.Input = append(s.Input, nil)
	s.ensureWords()
}

// Backspace deletes one character; on an empty word it steps back into the previous word only if that word was wrong.
func (s *Session) Backspace(now time.Time) {
	if s.finished || !s.started {
		return
	}
	if len(s.Input[s.Cur]) > 0 {
		s.Input[s.Cur] = s.Input[s.Cur][:len(s.Input[s.Cur])-1]
		s.log(now, Event{Kind: KindBackspace, Word: s.Cur, Pos: len(s.Input[s.Cur])})
		return
	}
	if s.Cur > 0 && !s.WordCorrect(s.Cur-1) {
		s.Input = s.Input[:s.Cur]
		s.Cur--
		s.log(now, Event{Kind: KindBackspace, Word: s.Cur, Pos: len(s.Input[s.Cur])})
	}
}

// DeleteWord clears the current word (or steps back into a wrong previous word).
func (s *Session) DeleteWord(now time.Time) {
	if s.finished || !s.started {
		return
	}
	if len(s.Input[s.Cur]) == 0 {
		if s.Cur > 0 && !s.WordCorrect(s.Cur-1) {
			s.Input = s.Input[:s.Cur]
			s.Cur--
		} else {
			return
		}
	}
	s.Input[s.Cur] = s.Input[s.Cur][:0]
	s.log(now, Event{Kind: KindDeleteWord, Word: s.Cur})
}

// End stops the session early (used by pace rounds and when quitting a running test).
func (s *Session) End(now time.Time) {
	if s.started {
		s.finish(now)
	}
}

func (s *Session) WordCorrect(i int) bool {
	return i < len(s.Input) && string(s.Input[i]) == string(s.Words[i])
}

// CaretOffset is the number of characters (including spaces) up to the caret, used for the pace ghost.
func (s *Session) CaretOffset() int {
	n := 0
	for i := 0; i < s.Cur; i++ {
		n += len(s.Words[i]) + 1
	}
	return n + min(len(s.Input[s.Cur]), len(s.Words[s.Cur]))
}

// Stats are Monkeytype-style results.
type Stats struct {
	WPM, Raw, Acc, Consistency        float64
	Correct, Incorrect, Extra, Missed int
	Duration                          time.Duration
	PerSecond                         []Second
	WordsTyped                        int
}

type Second struct {
	WPM, Raw float64
	Errors   int
}

// Stats scores the session. WPM counts characters of correctly typed words plus their spaces;
// raw counts everything typed; accuracy is correct keypresses over all keypresses.
func (s *Session) Stats() Stats {
	st := Stats{Duration: s.duration}
	if !s.finished {
		return st
	}
	mins := s.duration.Minutes()
	if mins <= 0 {
		return st
	}
	correctChars, rawChars := 0, 0
	last := len(s.Input) - 1
	for i := 0; i <= last; i++ {
		w, in := s.Words[i], s.Input[i]
		submitted := i < last || s.lastDone
		for j := 0; j < len(in); j++ {
			switch {
			case j >= len(w):
				st.Extra++
			case in[j] == w[j]:
				st.Correct++
			default:
				st.Incorrect++
			}
		}
		if submitted && len(in) < len(w) {
			st.Missed += len(w) - len(in)
		}
		rawChars += len(in)
		if submitted {
			rawChars++ // the space
			st.WordsTyped++
		}
		switch {
		case string(in) == string(w):
			correctChars += len(w)
			if submitted && i < len(s.Words)-1 {
				correctChars++
			}
		case !submitted && isPrefix(in, w):
			correctChars += len(in) // partial last word in a timed test
		}
	}
	st.WPM = float64(correctChars) / 5 / mins
	st.Raw = float64(rawChars) / 5 / mins
	keys, good := 0, 0
	for _, e := range s.Events {
		if e.Kind == KindChar || e.Kind == KindSpace {
			keys++
			if e.Correct {
				good++
			}
		}
	}
	if keys > 0 {
		st.Acc = 100 * float64(good) / float64(keys)
	}
	st.PerSecond = s.perSecond()
	st.Consistency = consistency(st.PerSecond)
	return st
}

func isPrefix(a, b []rune) bool {
	if len(a) > len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (s *Session) perSecond() []Second {
	secs := int(math.Ceil(s.duration.Seconds()))
	if secs < 1 {
		return nil
	}
	out := make([]Second, secs)
	raw := make([]int, secs)
	cumGood := 0
	ei := 0
	for sec := 0; sec < secs; sec++ {
		end := time.Duration(sec+1) * time.Second
		if end > s.duration {
			end = s.duration
		}
		for ei < len(s.Events) && s.Events[ei].T < end {
			e := s.Events[ei]
			if e.Kind == KindChar || e.Kind == KindSpace {
				raw[sec]++
				if e.Correct {
					cumGood++
				} else {
					out[sec].Errors++
				}
			}
			ei++
		}
		span := end.Seconds() - float64(sec)
		if span <= 0 {
			span = 1
		}
		out[sec].Raw = float64(raw[sec]) / 5 * 60 / span
		out[sec].WPM = float64(cumGood) / 5 / end.Minutes()
	}
	return out
}

// consistency follows Monkeytype: 100 × (1 − tanh(cv + cv³/3 + cv⁵/5)) over per-second raw speed.
func consistency(ps []Second) float64 {
	if len(ps) < 2 {
		return 100
	}
	var sum float64
	for _, p := range ps {
		sum += p.Raw
	}
	mean := sum / float64(len(ps))
	if mean == 0 {
		return 0
	}
	var v float64
	for _, p := range ps {
		v += (p.Raw - mean) * (p.Raw - mean)
	}
	cv := math.Sqrt(v/float64(len(ps))) / mean
	return 100 * (1 - math.Tanh(cv+cv*cv*cv/3+math.Pow(cv, 5)/5))
}
