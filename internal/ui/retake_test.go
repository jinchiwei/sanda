package ui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jinchiwei/sanda/internal/data"
	"github.com/jinchiwei/sanda/internal/engine"
)

func completeRetakeFixture(m *Model) {
	now := time.Now().Add(-5 * time.Second)
	for _, r := range "cat dog owl" {
		if r == ' ' {
			m.sess.Space(now)
		} else {
			m.sess.Type(r, now)
		}
		now = now.Add(200 * time.Millisecond)
	}
	m.checkDone()
	m.shownAt = time.Now().Add(-time.Second)
}

// Retaking repeats the same prompt but starts a distinct attempt without
// overwriting or duplicating the already saved result.
func TestRetakeRepeatsTextAndSavesSeparateAttempt(t *testing.T) {
	m := newTestModel(t)
	m.cfg.Mode, m.cfg.Words = "words", 3
	m.words = []string{"cat", "dog", "owl"}
	m.sess = engine.New(m.words)
	completeRetakeFixture(m)
	if m.screen != scrResult || !strings.Contains(m.viewResult(), "retake") {
		t.Fatal("results must offer retake")
	}
	press(m, "r")
	if m.screen != scrTyping || m.sess.Started() || m.saved || len(m.sess.Events) != 0 {
		t.Fatal("retake must start a fresh attempt")
	}
	if !reflect.DeepEqual(m.sess.Words, [][]rune{[]rune("cat"), []rune("dog"), []rune("owl")}) {
		t.Fatal("retake changed the prompt")
	}
	saved, err := m.db.Tests(time.Time{})
	if err != nil || len(saved) != 1 {
		t.Fatalf("retake changed saved results before completion: %v %v", saved, err)
	}
	completeRetakeFixture(m)
	saved, err = m.db.Tests(time.Time{})
	if err != nil || len(saved) != 2 || saved[0].Text != "cat dog owl" || saved[1].Text != "cat dog owl" {
		t.Fatalf("retake must save a separate result for the same text: %v %v", saved, err)
	}
}

// Timed retakes must preserve words appended after the initial batch, and pace
// retakes must not advance the phase or recalculate the target.
func TestRetakePreservesExpandedTextAndPaceSettings(t *testing.T) {
	m := newTestModel(t)
	m.activity = ActPace
	m.phase = phaseControl
	m.target = 145
	m.words = []string{"initial"}
	m.sess = engine.New([]string{"initial", "expanded", "tail"})
	m.sess.TimeLimit = 30 * time.Second
	m.sess.FreeBackspace = true
	m.sess.More = func(n int) []string { return []string{"more"} }
	m.sess.Type('i', time.Now())
	m.sess.End(time.Now().Add(time.Second))
	m.screen = scrResult
	m.shownAt = time.Now().Add(-time.Second)
	press(m, "r")
	if m.screen != scrTyping || m.sess.Started() || len(m.sess.Words) != 3 {
		t.Fatal("retake lost expanded text or failed to reset")
	}
	if m.phase != phaseControl || m.target != 145 || m.sess.TimeLimit != 30*time.Second || !m.sess.FreeBackspace || m.sess.More == nil {
		t.Fatal("retake changed pace settings")
	}
}

func TestRetakeKeepsQuoteAndSource(t *testing.T) {
	m := newTestModel(t)
	m.cfg.Mode = "quote"
	m.quote = &data.Quote{Text: "cat dog owl", Source: "original source"}
	quote := m.quote
	m.words = []string{"cat", "dog", "owl"}
	m.sess = engine.New(m.words)
	m.screen = scrResult
	m.shownAt = time.Now().Add(-time.Second)
	press(m, "r")
	if m.screen != scrTyping || m.quote != quote || m.quote.Source != "original source" {
		t.Fatal("retake replaced the quote or source")
	}
}
