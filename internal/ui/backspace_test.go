package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jinchiwei/sanda/internal/config"
	"github.com/jinchiwei/sanda/internal/engine"
)

// Terminals can encode ordinary Backspace as DEL or Ctrl+H. Neither may
// clear a word: crossing a boundary removes the space, then one rune per key.
func TestBackspaceDeletesOneCharacterAcrossWordBoundary(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyBackspace, tea.KeyCtrlH} {
		for _, word := range []string{"cat", "cxt", "café"} {
			t.Run(key.String()+"/"+word, func(t *testing.T) {
				m := newTestModel(t)
				m.sess = engine.New([]string{"cat", "dog", "tail"})
				m.sess.FreeBackspace = true
				now := time.Now()
				for _, r := range word {
					m.sess.Type(r, now)
				}
				m.sess.Space(now)
				m.Update(tea.KeyMsg{Type: key})
				if m.sess.Cur != 0 || string(m.sess.Input[0]) != word {
					t.Fatalf("first backspace must remove only the space: cur=%d input=%q", m.sess.Cur, m.sess.Input[0])
				}
				runes := []rune(word)
				for remaining := len(runes) - 1; remaining >= 0; remaining-- {
					m.Update(tea.KeyMsg{Type: key})
					if got := string(m.sess.Input[0]); got != string(runes[:remaining]) {
						t.Fatalf("backspace should remove one rune: got %q want %q", got, string(runes[:remaining]))
					}
				}
			})
		}
	}
}

func TestFreedomCtrlHDeletesOneCharacter(t *testing.T) {
	base := newTestModel(t)
	m := New(config.Default(), base.db, "freedom")
	press(m, "hello world ")
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlH})
	if got := string(m.free.text); got != "hello world" {
		t.Fatalf("backspace should remove only space, got %q", got)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlH})
	if got := string(m.free.text); got != "hello worl" {
		t.Fatalf("backspace should remove only one letter, got %q", got)
	}
}
