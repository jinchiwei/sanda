// Package train picks what to drill, builds drill text from real words and runs the spaced-repetition schedule.
package train

import (
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jinchiwei/sanda/internal/analysis"
	"github.com/jinchiwei/sanda/internal/data"
	"github.com/jinchiwei/sanda/internal/engine"
	"github.com/jinchiwei/sanda/internal/store"
	"github.com/jinchiwei/sanda/internal/textgen"
)

// Focus is a unit chosen for a drill round.
type Focus struct {
	Unit   string
	Probe  bool // not enough data yet; drilled to measure it
	Before time.Duration
}

// PickFocus returns up to n units: the costliest weaknesses that are due, plus one common unit that still lacks data.
func PickFocus(m analysis.Model, sched map[string]store.Sched, now time.Time, n int) []Focus {
	var out []Focus
	for _, u := range m.Ranked {
		if len(out) >= n {
			break
		}
		if s, ok := sched[u.Unit]; ok && s.Due.After(now) {
			continue // recently passed; not due yet
		}
		if !drillable(u.Unit) {
			continue
		}
		out = append(out, Focus{Unit: u.Unit, Before: u.Median})
	}
	// Probe: the most frequent transition we have little data on.
	var probes []*analysis.UnitStat
	for _, u := range m.Units {
		if u.Uncertain && u.Freq > 2 && drillable(u.Unit) {
			probes = append(probes, u)
		}
	}
	sort.Slice(probes, func(i, j int) bool { return probes[i].Freq > probes[j].Freq })
	if len(probes) > 0 && len(out) < n+1 {
		out = append(out, Focus{Unit: probes[0].Unit, Probe: true, Before: probes[0].Median})
	}
	return out
}

// drillable reports whether words can be built around a transition.
func drillable(unit string) bool {
	r := []rune(unit)
	if len(r) != 2 || (r[0] == ' ' && r[1] == ' ') {
		return false
	}
	for _, c := range r {
		if !(unicode.IsLetter(c) || unicode.IsDigit(c) || c == ' ' || unicode.IsPunct(c)) {
			return false
		}
	}
	return true
}

// Drill builds about n words that are dense in the focus units, with ~30% plain filler to keep a natural rhythm.
func Drill(focus []Focus, n int, seed uint64) []string {
	rng := rand.New(rand.NewPCG(seed, seed^0xd411))
	words := data.Words10k()
	filler := data.Words200()
	if len(focus) == 0 {
		out := make([]string, n)
		for i := range out {
			out[i] = filler[rng.IntN(len(filler))]
		}
		return out
	}
	var out []string
	for len(out) < n {
		if rng.IntN(10) < 3 {
			out = append(out, filler[rng.IntN(len(filler))])
			continue
		}
		f := focus[len(out)%len(focus)]
		out = append(out, wordFor(f.Unit, words, rng))
	}
	return out
}

// wordFor makes one word (possibly with punctuation or a capital) that contains the transition.
func wordFor(unit string, words []string, rng *rand.Rand) string {
	r := []rune(unit)
	a, b := r[0], r[1]
	pick := func(match func(string) bool) string {
		var hits []string
		for _, w := range words { // frequency-ordered, so early hits are common words
			if match(w) {
				hits = append(hits, w)
				if len(hits) >= 300 {
					break
				}
			}
		}
		if len(hits) == 0 {
			return words[rng.IntN(200)]
		}
		// Bias toward common words.
		i := int(float64(len(hits)) * rng.Float64() * rng.Float64())
		return hits[i]
	}
	lower := func(c rune) rune { return unicode.ToLower(c) }
	isWordChar := func(c rune) bool { return unicode.IsLetter(c) }
	switch {
	case a == ' ' && unicode.IsDigit(b): // number start
		return strconv.Itoa(int(b-'0')) + strconv.Itoa(rng.IntN(1000))
	case unicode.IsDigit(a) || unicode.IsDigit(b): // digit pair inside a number
		return digitsWith(a, b, rng)
	case a == ' ' && isWordChar(b): // word start, maybe a capital
		w := pick(func(w string) bool { return strings.HasPrefix(w, string(lower(b))) })
		if unicode.IsUpper(b) {
			w = textgen.Capitalize(w)
		}
		return w
	case isWordChar(a) && b == ' ': // word end
		return pick(func(w string) bool { return strings.HasSuffix(w, string(lower(a))) })
	case isWordChar(a) && isWordChar(b): // letter pair, maybe a capital
		w := pick(func(w string) bool { return strings.Contains(strings.ToLower(w), string([]rune{lower(a), lower(b)})) })
		if unicode.IsUpper(a) || unicode.IsUpper(b) {
			w = textgen.Capitalize(w)
		}
		return w
	case isWordChar(a) && unicode.IsPunct(b): // letter then punctuation, e.g. "e,"
		w := pick(func(w string) bool { return strings.HasSuffix(w, string(lower(a))) })
		if b == '\'' {
			return w + "'s"
		}
		return w + string(b)
	case unicode.IsPunct(a) && b == ' ': // punctuation then space
		return pick(func(string) bool { return true }) + string(a)
	case unicode.IsPunct(a) && isWordChar(b): // e.g. opening quote, "n't"
		if a == '\'' {
			return pick(func(w string) bool { return strings.HasSuffix(w, "n") }) + "'" + string(b)
		}
		return string(a) + pick(func(w string) bool { return strings.HasPrefix(w, string(lower(b))) })
	case a == ' ' && unicode.IsPunct(b): // space then opening punctuation
		return string(b) + pick(func(string) bool { return true })
	}
	return pick(func(w string) bool { return strings.ContainsRune(w, lower(a)) })
}

func digitsWith(a, b rune, rng *rand.Rand) string {
	var sb strings.Builder
	if unicode.IsDigit(a) {
		sb.WriteRune(a)
	} else {
		sb.WriteString(strconv.Itoa(1 + rng.IntN(9)))
	}
	if unicode.IsDigit(b) {
		sb.WriteRune(b)
	}
	sb.WriteString(strconv.Itoa(rng.IntN(100)))
	return sb.String()
}

// Outcome of one drilled unit in a round.
type Outcome struct {
	Unit     string
	Round    time.Duration
	Samples  int
	Before   time.Duration
	Baseline time.Duration
	Passed   bool
	NextDue  time.Time
	Level    int
}

// Review grades each focus unit after a round and updates its schedule: a pass (within 5% of your baseline)
// pushes the next review out 1, 2, 4, 8… days; a miss brings it back immediately.
func Review(db *store.DB, focus []Focus, events []engine.Event, baseline time.Duration, now time.Time) []Outcome {
	sched, _ := db.Schedule()
	var out []Outcome
	for _, f := range focus {
		med, n := analysis.RoundMedian(events, f.Unit)
		o := Outcome{Unit: f.Unit, Round: med, Samples: n, Before: f.Before, Baseline: baseline}
		s := sched[f.Unit]
		s.Unit = f.Unit
		if n < 2 {
			out = append(out, o) // not enough clean samples to grade
			continue
		}
		o.Passed = med <= baseline+baseline/20
		if o.Passed {
			s.Level++
			s.Due = now.Add(time.Duration(1<<(s.Level-1)) * 24 * time.Hour)
		} else {
			s.Level = 0
			s.Due = now
		}
		s.LastSeen = now
		db.SetSchedule(s)
		o.NextDue, o.Level = s.Due, s.Level
		out = append(out, o)
	}
	return out
}

// PaceTarget is the pacer speed: the median of your last 10 official tests × factor (60 WPM with no history).
func PaceTarget(tests []store.Test, factor float64) float64 {
	var w []float64
	for i := len(tests) - 1; i >= 0 && len(w) < 10; i-- {
		if tests[i].Kind == store.KindOfficial {
			w = append(w, tests[i].WPM)
		}
	}
	if len(w) == 0 {
		return 60
	}
	sort.Float64s(w)
	med := w[len(w)/2]
	if len(w)%2 == 0 {
		med = (w[len(w)/2-1] + w[len(w)/2]) / 2
	}
	return med * factor
}
