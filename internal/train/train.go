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

// Format is the kind of text a train round drills, matching the tests it's meant to improve. Each
// format learns only from its own tests: capitals and punctuation slow everything around them, so
// quote timings would make plain words look weak, and vice versa.
type Format string

const (
	Plain Format = "plain" // time and words tests: lowercase words, no punctuation
	Prose Format = "prose" // quotes, or punctuation on: capitals and punctuation included
)

// FormatFor is the format that trains for tests with these settings.
func FormatFor(mode string, punct bool) Format {
	if mode == "quote" || punct {
		return Prose
	}
	return Plain
}

// Name is how the format is shown.
func (f Format) Name() string {
	if f == Plain {
		return "time/words"
	}
	return "quotes"
}

// DrillMode is the mode a train round of this format is saved under.
func (f Format) DrillMode() string { return "drill-" + string(f) }

// Counts reports whether a saved round is part of this format's history. Rounds from before formats
// existed (mode "drill") count for neither: they mixed capitals into every round, which slowed the
// lowercase units drilled with them.
func (f Format) Counts(t store.Test) bool {
	switch t.Kind {
	case store.KindOfficial:
		return FormatFor(t.Mode, t.Punctuation) == f
	case store.KindTrain:
		return t.Mode == f.DrillMode()
	}
	return false
}

// SchedKey is the unit's key in the review schedule; each format keeps its own.
func (f Format) SchedKey(unit string) string { return string(f) + ":" + unit }

// allows reports whether the format's tests contain this kind of transition at all.
func (f Format) allows(c analysis.Class) bool {
	return f == Prose || c == analysis.ClassLetters || c == analysis.ClassWordStart || c == analysis.ClassSpace
}

// Focus is a unit chosen for a drill round.
type Focus struct {
	Unit   string
	Probe  bool // not enough data yet; drilled to measure it
	Before time.Duration
	Target time.Duration // par for this kind of transition; 0 means the overall median
}

// PickFocus returns up to n units: the costliest weaknesses that are due, plus one common unit that still lacks data.
// Capitals get a round to themselves. A capital-heavy round is slower all over, so drilling one alongside
// lowercase units would make those look worse than they are, and they'd never pass.
func PickFocus(m analysis.Model, sched map[string]store.Sched, now time.Time, n int, f Format) []Focus {
	var out []Focus
	for _, u := range m.Ranked {
		if len(out) >= n {
			break
		}
		if s, ok := sched[f.SchedKey(u.Unit)]; ok && s.Due.After(now) {
			continue // recently drilled; not due yet
		}
		if !drillable(u.Unit) || !f.allows(u.Class) {
			continue
		}
		if u.Class == analysis.ClassShift {
			if len(out) == 0 {
				return []Focus{{Unit: u.Unit, Before: u.Median, Target: u.Target}}
			}
			continue
		}
		out = append(out, Focus{Unit: u.Unit, Before: u.Median, Target: u.Target})
	}
	// Probe: the most frequent transition we have little data on.
	var probes []*analysis.UnitStat
	for _, u := range m.Units {
		if s, ok := sched[f.SchedKey(u.Unit)]; ok && s.Due.After(now) {
			continue
		}
		if u.Uncertain && u.Freq > 2 && drillable(u.Unit) && u.Class != analysis.ClassShift && f.allows(u.Class) {
			probes = append(probes, u)
		}
	}
	sort.Slice(probes, func(i, j int) bool { return probes[i].Freq > probes[j].Freq })
	if len(probes) > 0 && len(out) < n+1 {
		out = append(out, Focus{Unit: probes[0].Unit, Probe: true, Before: probes[0].Median, Target: probes[0].Target})
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
	// Lowercase words only: a capital or apostrophe comes from the focus unit, never by accident
	// (the top-200 list has "I", which would put Shift into a time/words drill).
	words, filler := lowercase(data.Words10k()), lowercase(data.Words200())
	if len(focus) == 0 {
		out := make([]string, n)
		for i := range out {
			out[i] = filler[rng.IntN(len(filler))]
		}
		return out
	}
	if r := []rune(focus[0].Unit); len(focus) == 1 && r[0] == ' ' && unicode.IsUpper(r[1]) {
		return capitalDrill(r[1], n, rng)
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

func lowercase(list []string) []string {
	var out []string
	for _, w := range list {
		if strings.IndexFunc(w, func(r rune) bool { return !unicode.IsLower(r) }) < 0 {
			out = append(out, w)
		}
	}
	return out
}

// capitalDrill drills a capital word start the way quotes use one: short sentences that each open
// with it, from common words ("It", "In", "I'm"), so it reads like text rather than a list of names.
func capitalDrill(c rune, n int, rng *rand.Rand) []string {
	filler := lowercase(data.Words200())
	var starts []string
	for _, w := range filler {
		if strings.HasPrefix(w, string(unicode.ToLower(c))) {
			starts = append(starts, textgen.Capitalize(w))
		}
	}
	if c == 'I' { // in quotes "I" is mostly the pronoun
		starts = append(starts, "I", "I", "I", "I'm", "I've", "I'll", "I'd")
	}
	if len(starts) == 0 {
		starts = []string{string(c)}
	}
	var out []string
	for len(out) < n {
		out = append(out, starts[rng.IntN(len(starts))])
		for range 1 + rng.IntN(3) {
			out = append(out, filler[rng.IntN(len(filler))])
		}
		out[len(out)-1] += "."
	}
	return out[:n]
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
	Unit    string
	Round   time.Duration
	Samples int
	Before  time.Duration
	Target  time.Duration // par it was graded against
	Passed  bool
	NextDue time.Time
	Level   int
}

// Retry is how long a unit that missed rests before it's drilled again, so one stubborn unit can't
// take every round: the next few rounds rotate through other weak spots, then it comes back.
const Retry = 20 * time.Minute

// Review grades each focus unit after a round and updates its schedule: a pass (within 5% of par for
// its kind of transition) pushes the next review out 1, 2, 4, 8… days; a miss rests it for Retry.
func Review(db *store.DB, focus []Focus, events []engine.Event, baseline time.Duration, now time.Time, fm Format) []Outcome {
	sched, _ := db.Schedule()
	var out []Outcome
	for _, f := range focus {
		med, n := analysis.RoundMedian(events, f.Unit)
		target := f.Target
		if target == 0 {
			target = baseline
		}
		o := Outcome{Unit: f.Unit, Round: med, Samples: n, Before: f.Before, Target: target}
		key := fm.SchedKey(f.Unit)
		s := sched[key]
		s.Unit = key
		if n < 2 {
			out = append(out, o) // not enough clean samples to grade
			continue
		}
		o.Passed = med <= analysis.Par(target)
		if o.Passed {
			s.Level++
			s.Due = now.Add(time.Duration(1<<(s.Level-1)) * 24 * time.Hour)
		} else {
			s.Level = 0
			s.Due = now.Add(Retry)
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
