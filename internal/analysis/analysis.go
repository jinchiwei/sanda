// Package analysis turns raw keystrokes into transition timings, a per-test "what cost you" breakdown
// and a long-term weakness model.
package analysis

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/jinchiwei/sanda/internal/data"
	"github.com/jinchiwei/sanda/internal/engine"
	"github.com/jinchiwei/sanda/internal/store"
)

// Pauses longer than this are breaks, not typing, and are left out of timing stats.
const maxIKI = 1500 * time.Millisecond

// Transition is a clean (first-try, correct, after a correct key) move from one character to the next.
type Transition struct {
	From, To rune
	IKI      time.Duration
	At       time.Duration
	Word     int
}

func (t Transition) Unit() string { return string([]rune{t.From, t.To}) }

// Miss is a first-try error on a transition.
type Miss struct{ From, To rune }

func (m Miss) Unit() string { return string([]rune{m.From, m.To}) }

// Transitions extracts clean transitions and first-try misses from one round's keystrokes.
func Transitions(events []engine.Event) (clean []Transition, misses []Miss) {
	for i := 1; i < len(events); i++ {
		e, p := events[i], events[i-1]
		if e.Kind != engine.KindChar && e.Kind != engine.KindSpace {
			continue
		}
		prevOK := (p.Kind == engine.KindChar || p.Kind == engine.KindSpace) && p.Correct && p.First
		if !prevOK || !e.First || e.Expected == 0 || p.Expected == 0 {
			continue
		}
		if !e.Correct {
			if e.Kind == engine.KindChar {
				misses = append(misses, Miss{p.Expected, e.Expected})
			}
			continue
		}
		iki := e.T - p.T
		if iki <= 0 || iki > maxIKI {
			continue
		}
		clean = append(clean, Transition{From: p.Expected, To: e.Expected, IKI: iki, At: e.T, Word: e.Word})
	}
	return clean, misses
}

// Class groups transitions by what the target key needs.
type Class string

const (
	ClassShift     Class = "Shift (capitals, shifted symbols)"
	ClassPunct     Class = "punctuation"
	ClassDigit     Class = "numbers"
	ClassWordStart Class = "word starts (space → letter)"
	ClassSpace     Class = "spaces (letter → space)"
	ClassLetters   Class = "letter pairs"
)

const shifted = "~!@#$%^&*()_+{}|:\"<>?"

func Classify(from, to rune) Class {
	switch {
	case unicode.IsUpper(to) || strings.ContainsRune(shifted, to):
		return ClassShift
	case unicode.IsDigit(to):
		return ClassDigit
	case to == ' ':
		return ClassSpace
	case unicode.IsPunct(to) || unicode.IsSymbol(to):
		return ClassPunct
	case from == ' ':
		return ClassWordStart
	}
	return ClassLetters
}

func median(d []time.Duration) time.Duration {
	if len(d) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), d...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

func ikis(ts []Transition) []time.Duration {
	out := make([]time.Duration, len(ts))
	for i, t := range ts {
		out[i] = t.IKI
	}
	return out
}

// Show makes a unit printable: spaces become ␣.
func Show(unit string) string { return strings.ReplaceAll(unit, " ", "␣") }

// Loss is an estimate of time lost to one cause in a single test.
type Loss struct {
	Label   string
	Seconds float64
}

// Breakdown estimates where time went in one test, compared with that test's own median transition.
// Categories overlap (a capital in a rare word counts twice), so they are estimates, not a sum.
func Breakdown(events []engine.Event, words []string, duration time.Duration) []Loss {
	clean, _ := Transitions(events)
	if len(clean) < 10 {
		return nil
	}
	base := median(ikis(clean))
	over := func(t Transition) float64 {
		if t.IKI > base {
			return (t.IKI - base).Seconds()
		}
		return 0
	}
	byClass := map[Class]float64{}
	byPair := map[string]float64{}
	rare := 0.0
	for _, t := range clean {
		c := Classify(t.From, t.To)
		byClass[c] += over(t)
		if c == ClassLetters {
			byPair[t.Unit()] += over(t)
		}
		if t.Word < len(words) && t.To != ' ' && !data.IsCommon(strings.Trim(words[t.Word], ".,;:!?\"'()-")) {
			rare += over(t)
		}
	}
	var out []Loss
	for _, c := range []Class{ClassShift, ClassPunct, ClassDigit, ClassWordStart} {
		if byClass[c] > 0 {
			out = append(out, Loss{string(c), byClass[c]})
		}
	}
	if rare > 0 {
		out = append(out, Loss{"rarer words (outside the top 1000)", rare})
	}
	// Corrections: every keystroke that isn't clean forward progress.
	corr := 0.0
	for i := 1; i < len(events); i++ {
		e := events[i]
		if e.Kind == engine.KindBackspace || e.Kind == engine.KindDeleteWord || !e.Correct || !e.First {
			if d := e.T - events[i-1].T; d > 0 && d <= maxIKI {
				corr += d.Seconds()
			}
		}
	}
	if corr > 0 {
		out = append(out, Loss{"errors and corrections", corr})
	}
	// Pace drift: the last third slower than the first third.
	if duration >= 20*time.Second && len(clean) >= 30 {
		third := len(clean) / 3
		first, last := median(ikis(clean[:third])), median(ikis(clean[len(clean)-third:]))
		if last > first {
			out = append(out, Loss{"slowing down over the test", float64(third) * (last - first).Seconds()})
		}
	}
	// The single slowest letter pair.
	var worst string
	for u, s := range byPair {
		if worst == "" || s > byPair[worst] {
			worst = u
		}
	}
	if worst != "" && byPair[worst] > 0 {
		out = append(out, Loss{fmt.Sprintf("slowest pair %q", worst), byPair[worst]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seconds > out[j].Seconds })
	return out
}

// Corpus frequency of every transition in Monkeytype's quotes, per 1000 transitions.
var (
	freqOnce sync.Once
	freq     map[string]float64
)

func Freq(unit string) float64 {
	freqOnce.Do(func() {
		counts := map[string]int{}
		total := 0
		for _, q := range data.Quotes(data.QuoteAll) {
			r := []rune(strings.Join(strings.Fields(q.Text), " "))
			for i := 1; i < len(r); i++ {
				counts[string(r[i-1:i+1])]++
				total++
			}
		}
		freq = make(map[string]float64, len(counts))
		for k, v := range counts {
			freq[k] = 1000 * float64(v) / float64(total)
		}
	})
	return freq[unit]
}

// UnitStat is everything known about one transition across history.
type UnitStat struct {
	Unit      string
	Count     int
	Median    time.Duration // over the most recent samples
	Errors    int
	ErrRate   float64
	Days      int
	Daily     []Day // oldest first
	Freq      float64
	Class     Class
	Target    time.Duration // par: your median for this kind of transition
	Cost      float64       // estimated seconds lost per 1000 characters of typical text
	Uncertain bool
}

type Day struct {
	Date   string
	Median time.Duration
}

type ClassStat struct {
	Class  Class
	Count  int
	Median time.Duration
}

// Model is the long-term weakness model.
type Model struct {
	Baseline time.Duration
	Units    map[string]*UnitStat
	Ranked   []*UnitStat // by cost, highest first, certain units only
	Classes  []ClassStat
	ClassMed map[Class]time.Duration
	ClassErr map[Class]float64
	Samples  int
}

const (
	recentSamples = 60
	minSamples    = 6
	// A unit is a weak spot only when it's this much slower than par, or makes this many more
	// errors than its kind of transition does. Review passes it on the same terms.
	slowMargin = 0.05
	errMargin  = 0.01
	// A capital is held to the same move in lowercase plus this much for Shift. Judged against
	// other capitals instead, the commonest one (I) looks fine while costing you on every "I".
	shiftAllowance = 0.4
)

// Par is what a unit is held to: its class median plus the margin.
func Par(target time.Duration) time.Duration {
	return target + time.Duration(float64(target)*slowMargin)
}

// BuildModel aggregates every round's keystrokes (official, train and pace all count here).
func BuildModel(rounds []store.TestEvents) Model {
	type acc struct {
		ikis  []time.Duration
		errs  int
		days  map[string][]time.Duration
		order []string
	}
	units := map[string]*acc{}
	get := func(u string) *acc {
		a := units[u]
		if a == nil {
			a = &acc{days: map[string][]time.Duration{}}
			units[u] = a
		}
		return a
	}
	var all []time.Duration
	classes := map[Class][]time.Duration{}
	classMiss := map[Class]int{}
	for _, r := range rounds {
		day := r.Test.StartedAt.Local().Format("2006-01-02")
		clean, misses := Transitions(r.Events)
		for _, t := range clean {
			a := get(t.Unit())
			a.ikis = append(a.ikis, t.IKI)
			if _, ok := a.days[day]; !ok {
				a.order = append(a.order, day)
			}
			a.days[day] = append(a.days[day], t.IKI)
			all = append(all, t.IKI)
			c := Classify(t.From, t.To)
			classes[c] = append(classes[c], t.IKI)
		}
		for _, m := range misses {
			get(m.Unit()).errs++
			classMiss[Classify(m.From, m.To)]++
		}
	}
	m := Model{Units: map[string]*UnitStat{}, ClassMed: map[Class]time.Duration{}, ClassErr: map[Class]float64{}, Samples: len(all)}
	m.Baseline = median(all)
	// Each unit is judged against its own kind of transition: a word start always takes longer
	// than a letter pair inside a word, and a capital longer still, so comparing them with the
	// overall median would flag the commonest word starts and capitals forever.
	for c, v := range classes {
		m.ClassMed[c] = median(v)
		m.ClassErr[c] = float64(classMiss[c]) / float64(len(v)+classMiss[c])
	}
	for u, a := range units {
		recent := a.ikis
		if len(recent) > recentSamples {
			recent = recent[len(recent)-recentSamples:]
		}
		r := []rune(u)
		c := Classify(r[0], r[1])
		s := &UnitStat{Unit: u, Count: len(a.ikis), Median: median(recent), Errors: a.errs, Days: len(a.days), Freq: Freq(u), Class: c, Target: m.ClassMed[c]}
		if lc := unicode.ToLower(r[1]); c == ClassShift && lc != r[1] {
			if base := m.ClassMed[Classify(r[0], lc)]; base > 0 {
				s.Target = min(s.Target, base+time.Duration(float64(base)*shiftAllowance))
			}
		}
		if s.Target == 0 {
			s.Target = m.Baseline
		}
		if tries := len(a.ikis) + a.errs; tries > 0 {
			s.ErrRate = float64(a.errs) / float64(tries)
		}
		for _, d := range a.order {
			s.Daily = append(s.Daily, Day{d, median(a.days[d])})
		}
		s.Uncertain = s.Count < minSamples
		slow := 0.0
		if s.Median > Par(s.Target) {
			slow = (s.Median - s.Target).Seconds()
		}
		errs := 0.0
		if extra := s.ErrRate - m.ClassErr[c]; extra > errMargin {
			errs = extra
		}
		// An error costs roughly four keystrokes: the wrong key, backspace, the retype and the hesitation.
		s.Cost = s.Freq * (slow + errs*4*s.Target.Seconds())
		m.Units[u] = s
		if !s.Uncertain && s.Cost > 0 {
			m.Ranked = append(m.Ranked, s)
		}
	}
	sort.Slice(m.Ranked, func(i, j int) bool { return m.Ranked[i].Cost > m.Ranked[j].Cost })
	for _, c := range []Class{ClassLetters, ClassWordStart, ClassSpace, ClassShift, ClassPunct, ClassDigit} {
		if v := classes[c]; len(v) > 0 {
			m.Classes = append(m.Classes, ClassStat{c, len(v), median(v)})
		}
	}
	return m
}

// RoundMedian is a unit's median in one round (0 if it never came up cleanly).
func RoundMedian(events []engine.Event, unit string) (time.Duration, int) {
	clean, _ := Transitions(events)
	var v []time.Duration
	for _, t := range clean {
		if t.Unit() == unit {
			v = append(v, t.IKI)
		}
	}
	return median(v), len(v)
}

// TopCorpusUnits returns the n most frequent letter-pair transitions in the quotes (used before any history exists).
func TopCorpusUnits(n int) []string {
	Freq("")
	var units []string
	for u := range freq {
		r := []rune(u)
		if unicode.IsLower(r[0]) && unicode.IsLower(r[1]) {
			units = append(units, u)
		}
	}
	sort.Slice(units, func(i, j int) bool { return freq[units[i]] > freq[units[j]] })
	return units[:min(n, len(units))]
}
