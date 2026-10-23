package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/jinchiwei/sanda/internal/engine"
)

func typingPromptRows(m *Model) []string {
	lines := strings.Split(ansi.Strip(m.viewTyping()), "\n")
	var rows []string
	for _, i := range []int{2, 4, 6} {
		rows = append(rows, strings.TrimRight(lines[i], " "))
	}
	return rows
}

func TestTypingWrapIncludesExtraMistypedLetters(t *testing.T) {
	m := newTestModel(t)
	m.w = 28 // 20 prompt columns
	m.sess = engine.New([]string{"abc", "def", "ghijklmnopq", "tail"})
	now := time.Now()
	for _, r := range "abcXX" {
		m.sess.Type(r, now)
	}
	m.sess.Space(now)
	rows := typingPromptRows(m)
	if rows[0] != "abcXX def" || rows[1] != "ghijklmnopq tail" {
		t.Fatalf("extra letters must wrap the final word intact, got %q", rows)
	}
	m.sess.Backspace(now)
	m.sess.Backspace(now)
	m.sess.Backspace(now)
	rows = typingPromptRows(m)
	if rows[0] != "abc def ghijklmnopq" {
		t.Fatalf("correcting extras should restore normal wrapping, got %q", rows)
	}
}

func TestTypingLongWordContinuesWithoutLosingLetters(t *testing.T) {
	m := newTestModel(t)
	m.w = 28
	word := "abcdefghijklmnopqrstuvwxyz"
	m.sess = engine.New([]string{word, "tail"})
	rows := typingPromptRows(m)
	if rows[0] != word[:20] || rows[1] != word[20:]+" tail" {
		t.Fatalf("long prompt must continue on next line, got %q", rows)
	}
	for _, r := range word {
		m.sess.Type(r, time.Now())
	}
	rows = typingPromptRows(m)
	if rows[0] != word[:20] || rows[1] != word[20:]+" tail" {
		t.Fatalf("typing must not hide the prompt continuation, got %q", rows)
	}
}

func TestTypingExtraLettersAtRightEdgeRemainVisible(t *testing.T) {
	m := newTestModel(t)
	m.w = 28
	word := "abcdefghijklmnopqrst"
	m.sess = engine.New([]string{word, "tail"})
	for _, r := range word + "XYZ" {
		m.sess.Type(r, time.Now())
	}
	rows := typingPromptRows(m)
	if rows[0] != word || rows[1] != "XYZ tail" {
		t.Fatalf("overflow input must stay visible, got %q", rows)
	}
	for _, row := range rows {
		if lipgloss.Width(row) > m.textWidth() {
			t.Fatalf("row exceeds typing area: %q", row)
		}
	}
}

func TestTypingPromptWrapUsesTerminalColumnsBeforeInput(t *testing.T) {
	m := newTestModel(t)
	m.w = 28
	m.sess = engine.New([]string{"abc", "界", "abcdefghijklmn", "tail"})
	rows := typingPromptRows(m)
	if rows[0] != "abc 界" || rows[1] != "abcdefghijklmn tail" {
		t.Fatalf("untouched words must fit in terminal columns, got %q", rows)
	}
	for _, row := range rows {
		if lipgloss.Width(row) > m.textWidth() {
			t.Fatalf("untouched prompt exceeds its area: %q", row)
		}
	}
}

func TestTypingCaretContinuesAfterFullWidthWord(t *testing.T) {
	m := newTestModel(t)
	m.w = 28
	word := "abcdefghijklmnopqrst"
	m.sess = engine.New([]string{word, "tail"})
	for _, r := range word {
		m.sess.Type(r, time.Now())
	}
	rows, current := m.typingRows(m.textWidth(), -1)
	if current != 1 || len(rows[1]) == 0 || rows[1][0].bg != m.th.Caret {
		t.Fatalf("caret must remain visible after a full-width word: row=%d cells=%v", current, rows)
	}
}

func TestTypingLongWordKeepsMistypeHintsAlignedAfterWrap(t *testing.T) {
	m := newTestModel(t)
	m.w = 28
	m.sess = engine.New([]string{"abcdefghijklmnopqrstuvwxyz", "tail"})
	for _, r := range "abcdefghijklmnopqrstX" {
		m.sess.Type(r, time.Now())
	}
	lines := strings.Split(ansi.Strip(m.viewTyping()), "\n")
	if strings.TrimRight(lines[4], " ") != "uvwxyz tail" || strings.TrimRight(lines[5], " ") != "X" {
		t.Fatalf("continuation must retain expected text and aligned hint: %q / %q", lines[4], lines[5])
	}
}
