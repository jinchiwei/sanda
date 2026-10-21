package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/jinchiwei/sanda/internal/engine"
)

// Mistakes must be aligned under the expected letter, not replace the prompt.
func TestMistypedLettersAppearBelowAndDisappearWhenCorrected(t *testing.T) {
	m := newTestModel(t)
	m.sess = engine.New([]string{"cat", "dog", "tail"})
	now := time.Now()
	for _, r := range "cxt" {
		m.sess.Type(r, now)
	}
	m.sess.Space(now)
	for _, r := range "dp" {
		m.sess.Type(r, now)
	}
	lines := strings.Split(ansi.Strip(m.viewTyping()), "\n")
	if strings.TrimRight(lines[2], " ") != "cat dog tail" {
		t.Fatalf("expected prompt remains visible: %q", lines[2])
	}
	if strings.TrimRight(lines[3], " ") != " x   p" {
		t.Fatalf("mistyped x and p should be aligned below a and o: %q", lines[3])
	}
	m.sess.Backspace(now)
	m.sess.Type('o', now)
	lines = strings.Split(ansi.Strip(m.viewTyping()), "\n")
	if strings.TrimRight(lines[3], " ") != " x" {
		t.Fatalf("corrected letter should disappear: %q", lines[3])
	}
}

// Every wrapped prompt line needs its own error line; empty rows must not jump.
func TestMistypedLettersFollowWrappingWithStableHeight(t *testing.T) {
	m := newTestModel(t)
	m.w = 28 // text width 20
	m.sess = engine.New([]string{"abcdefghijklmnop", "target", "end"})
	before := len(strings.Split(m.viewTyping(), "\n"))
	now := time.Now()
	for _, r := range "abcdefghijklmnoX" {
		m.sess.Type(r, now)
	}
	m.sess.Space(now)
	m.sess.Type('Z', now)
	lines := strings.Split(ansi.Strip(m.viewTyping()), "\n")
	if len(lines) != before {
		t.Fatalf("errors changed layout height: %d to %d", before, len(lines))
	}
	if strings.TrimRight(lines[3], " ") != "               X" || strings.TrimRight(lines[5], " ") != "Z" {
		t.Fatalf("errors must follow their wrapped lines: %q", lines)
	}
}

// An early space must be visible at the first missing letter, and disappear
// when the user returns to finish that word.
func TestEarlySpaceAppearsBelowFirstSkippedLetter(t *testing.T) {
	m := newTestModel(t)
	m.sess = engine.New([]string{"hello", "world"})
	now := time.Now()
	m.sess.Space(now) // empty words cannot be submitted
	if strings.Contains(ansi.Strip(m.viewTyping()), "␣") {
		t.Fatal("ignored space created a hint")
	}
	m.sess.Type('h', now)
	m.sess.Type('e', now)
	m.sess.Space(now)
	lines := strings.Split(ansi.Strip(m.viewTyping()), "\n")
	if strings.TrimRight(lines[3], " ") != "  ␣" {
		t.Fatalf("space should appear under the first l: %q", lines[3])
	}
	if !m.wordCells(0, 0, -1)[2].under {
		t.Fatal("incomplete word should remain underlined")
	}
	m.sess.Backspace(now)
	if strings.Contains(ansi.Strip(m.viewTyping()), "␣") {
		t.Fatal("returning to edit the word should clear the hint")
	}
	for _, r := range "llo" {
		m.sess.Type(r, now)
	}
	m.sess.Space(now)
	if strings.Contains(ansi.Strip(m.viewTyping()), "␣") {
		t.Fatal("a correctly submitted word should have no space hint")
	}
}
