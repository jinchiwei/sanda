// Package data embeds the Monkeytype quotes, word lists and themes (GPL-3.0, see assets/ATTRIBUTION.md).
package data

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed assets/*.json
var assets embed.FS

type Quote struct {
	ID     int    `json:"id"`
	Text   string `json:"text"`
	Source string `json:"source"`
	Length int    `json:"length"`
}

// QuoteLength names Monkeytype's quote length groups.
type QuoteLength string

const (
	QuoteShort  QuoteLength = "short"
	QuoteMedium QuoteLength = "medium"
	QuoteLong   QuoteLength = "long"
	QuoteThicc  QuoteLength = "thicc"
	QuoteAll    QuoteLength = "all"
)

var QuoteLengths = []QuoteLength{QuoteShort, QuoteMedium, QuoteLong, QuoteThicc}

type quoteFile struct {
	Groups [][2]int `json:"groups"`
	Quotes []Quote  `json:"quotes"`
}

type wordFile struct {
	Words []string `json:"words"`
}

var (
	loadOnce sync.Once
	loadErr  error
	quotes   quoteFile
	words200 []string
	words1k  []string
	words10k []string
	common   map[string]bool
)

func load() {
	loadOnce.Do(func() {
		if loadErr = readJSON("assets/quotes_english.json", &quotes); loadErr != nil {
			return
		}
		var w wordFile
		if loadErr = readJSON("assets/english_200.json", &w); loadErr != nil {
			return
		}
		words200 = w.Words
		w = wordFile{}
		if loadErr = readJSON("assets/english_1k.json", &w); loadErr != nil {
			return
		}
		words1k = w.Words
		w = wordFile{}
		if loadErr = readJSON("assets/english_10k.json", &w); loadErr != nil {
			return
		}
		words10k = w.Words
		common = make(map[string]bool, len(words1k))
		for _, x := range words1k {
			common[strings.ToLower(x)] = true
		}
	})
}

func readJSON(path string, v any) error {
	b, err := assets.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// Err reports whether the embedded data failed to load.
func Err() error { load(); return loadErr }

// Quotes returns every quote in the given length group (QuoteAll for all).
func Quotes(l QuoteLength) []Quote {
	load()
	if l == QuoteAll || l == "" {
		return quotes.Quotes
	}
	idx := map[QuoteLength]int{QuoteShort: 0, QuoteMedium: 1, QuoteLong: 2, QuoteThicc: 3}[l]
	g := quotes.Groups[idx]
	var out []Quote
	for _, q := range quotes.Quotes {
		if q.Length >= g[0] && q.Length <= g[1] {
			out = append(out, q)
		}
	}
	return out
}

// QuoteByID returns the quote with the given Monkeytype id.
func QuoteByID(id int) (Quote, bool) {
	load()
	for _, q := range quotes.Quotes {
		if q.ID == id {
			return q, true
		}
	}
	return Quote{}, false
}

// Words200 is Monkeytype's default English list (used for time and word tests).
func Words200() []string { load(); return words200 }

// Words10k is frequency-ordered and used to build drills.
func Words10k() []string { load(); return words10k }

// IsCommon reports whether a word is in the 1k most common English words.
func IsCommon(w string) bool { load(); return common[strings.ToLower(w)] }

// ThemesJSON returns the raw Monkeytype theme table.
func ThemesJSON() []byte {
	b, _ := assets.ReadFile("assets/monkeytype_themes.json")
	return b
}
