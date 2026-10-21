package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/jinchiwei/sanda/internal/data"
	"github.com/jinchiwei/sanda/internal/engine"
)

// typeQuote finishes a "cat dog owl" quote test with the given gap between keys and returns the results text.
func typeQuote(m *Model, gap time.Duration) string {
	m.cfg.Mode = "quote"
	m.activity = ActTest
	m.quote = &data.Quote{ID: 4242, Text: "cat dog owl", Source: "fixture"}
	m.words = []string{"cat", "dog", "owl"}
	m.sess = engine.New(m.words)
	m.saved = false
	m.screen = scrTyping
	now := time.Now().Add(-10 * time.Second)
	for _, r := range "cat dog owl" {
		if r == ' ' {
			m.sess.Space(now)
		} else {
			m.sess.Type(r, now)
		}
		now = now.Add(gap)
	}
	m.checkDone()
	return ansi.Strip(m.viewResult())
}

func TestQuoteRecordAcrossAttempts(t *testing.T) {
	m := newTestModel(t)
	if out := typeQuote(m, 200*time.Millisecond); !strings.Contains(out, "this quote  first attempt") {
		t.Fatalf("first attempt not shown:\n%s", out)
	}
	if out := typeQuote(m, 100*time.Millisecond); !strings.Contains(out, "attempt 2 · ★ new best on this quote") {
		t.Fatalf("faster second attempt should be a new quote best:\n%s", out)
	}
	if out := typeQuote(m, 300*time.Millisecond); !strings.Contains(out, "attempt 3 · best ") {
		t.Fatalf("slower third attempt should show the best so far:\n%s", out)
	}
	// History reopens attempt 2 with the record as it stood then.
	m.openHistory(scrMenu)
	for i, ht := range m.histTests {
		if ht.QuoteID == 4242 && i == 1 { // newest first: index 1 is attempt 2
			m.histIdx, m.histDetail = i, true
		}
	}
	if out := ansi.Strip(m.viewHistory()); !strings.Contains(out, "attempt 2 · ★ new best on this quote") {
		t.Fatalf("history detail should show the record at that attempt:\n%s", out)
	}
}
