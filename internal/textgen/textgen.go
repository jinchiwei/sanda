// Package textgen builds the words for time/word tests, with optional Monkeytype-style punctuation and numbers.
package textgen

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jinchiwei/sanda/internal/data"
)

type Gen struct {
	Punctuation bool
	Numbers     bool
	rng         *rand.Rand
	sinceStop   int // words since the last sentence end
	sentenceLen int
	prev        string
}

func New(punct, numbers bool, seed uint64) *Gen {
	g := &Gen{Punctuation: punct, Numbers: numbers, rng: rand.New(rand.NewPCG(seed, seed^0x5a5a))}
	g.newSentence()
	return g
}

func (g *Gen) newSentence() {
	g.sinceStop = 0
	g.sentenceLen = 4 + g.rng.IntN(9)
}

// Words returns n test words drawn from Monkeytype's top-200 English list, never repeating the previous word.
func (g *Gen) Words(n int) []string {
	list := data.Words200()
	out := make([]string, 0, n)
	for len(out) < n {
		w := list[g.rng.IntN(len(list))]
		if w == g.prev {
			continue
		}
		g.prev = w
		if g.Numbers && g.rng.IntN(10) == 0 {
			out = append(out, g.number())
			continue
		}
		if g.Punctuation {
			w = g.punctuate(w)
		}
		out = append(out, w)
	}
	return out
}

func (g *Gen) number() string {
	digits := 1 + g.rng.IntN(4)
	n := g.rng.IntN(pow10(digits))
	return strconv.Itoa(n)
}

func pow10(n int) int {
	p := 1
	for range n {
		p *= 10
	}
	return p
}

// punctuate capitalises sentence starts and adds commas, sentence ends, quotes, brackets, dashes and apostrophes.
func (g *Gen) punctuate(w string) string {
	if g.sinceStop == 0 {
		w = Capitalize(w)
	}
	g.sinceStop++
	if g.sinceStop >= g.sentenceLen {
		g.newSentence()
		ends := []string{".", ".", ".", ".", "?", "!"}
		return w + ends[g.rng.IntN(len(ends))]
	}
	switch r := g.rng.IntN(100); {
	case r < 9:
		return w + ","
	case r < 11:
		return w + ";"
	case r < 12:
		return w + ":"
	case r < 14:
		return "\"" + w + "\""
	case r < 15:
		return "(" + w + ")"
	case r < 16:
		return w + "'s"
	case r < 17:
		return "-"
	}
	return w
}

// Capitalize upper-cases the first letter.
func Capitalize(w string) string {
	r, size := utf8.DecodeRuneInString(w)
	if r == utf8.RuneError {
		return w
	}
	return string(unicode.ToUpper(r)) + w[size:]
}

// SplitQuote splits quote text into words the way the engine expects (single spaces).
func SplitQuote(text string) []string {
	return strings.Fields(text)
}
