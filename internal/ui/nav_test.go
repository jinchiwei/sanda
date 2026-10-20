package ui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jinchiwei/sanda/internal/config"
	"github.com/jinchiwei/sanda/internal/store"
)

func newTestModel(t *testing.T) *Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // settings saves go to a temp dir
	db, err := store.Open(filepath.Join(t.TempDir(), "sanda.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m := New(config.Default(), db, "")
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

func press(m *Model, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		m.Update(msg)
	}
}

// rowOf finds the menu row whose first item starts with label.
func rowOf(t *testing.T, m *Model, label string) int {
	for i, r := range m.menuRows {
		if len(r.items[0].label) >= len(label) && r.items[0].label[:len(label)] == label {
			return i
		}
	}
	t.Fatalf("no menu row %q", label)
	return 0
}

func goToRow(m *Model, row int) {
	for m.menuRow != row {
		press(m, "j")
	}
}

func TestEscReturnsToTestAfterThemePicker(t *testing.T) {
	m := newTestModel(t)
	press(m, "esc")
	if m.screen != scrMenu {
		t.Fatal("esc should open the menu")
	}
	goToRow(m, rowOf(t, m, "theme"))
	press(m, "enter")
	if m.screen != scrThemes {
		t.Fatal("enter on theme should open the picker")
	}
	press(m, "down", "enter")
	if m.screen != scrMenu {
		t.Fatalf("keeping a theme should return to the menu, got %v", m.screen)
	}
	press(m, "esc")
	if m.screen != scrTyping {
		t.Fatalf("esc in the menu should return to the test, got %v", m.screen)
	}
}

func TestEscReturnsAfterProgressAndThemeCancel(t *testing.T) {
	m := newTestModel(t)
	press(m, "esc")
	goToRow(m, rowOf(t, m, "progress"))
	press(m, "enter")
	m.View() // progress builds lazily
	press(m, "esc")
	if m.screen != scrMenu {
		t.Fatalf("esc in progress should return to the menu, got %v", m.screen)
	}
	goToRow(m, rowOf(t, m, "theme"))
	press(m, "enter", "esc")
	if m.screen != scrMenu {
		t.Fatalf("cancelling the theme picker should return to the menu, got %v", m.screen)
	}
	press(m, "esc")
	if m.screen != scrTyping {
		t.Fatalf("esc should get back to the test, got %v", m.screen)
	}
}

func TestEscMidTestStartsFresh(t *testing.T) {
	m := newTestModel(t)
	press(m, "a")
	if !m.sess.Started() {
		t.Fatal("typing should start the test")
	}
	press(m, "esc", "esc")
	if m.screen != scrTyping || m.sess.Started() {
		t.Fatal("returning from the menu mid-test should start a fresh test")
	}
}

func TestSelectingAModeStartsThatTest(t *testing.T) {
	m := newTestModel(t)
	press(m, "esc")
	goToRow(m, rowOf(t, m, "all")) // quote row
	press(m, "enter")
	if m.screen != scrTyping || m.cfg.Mode != "quote" || m.quote == nil {
		t.Fatalf("choosing a quote length should start a quote test (screen %v, mode %s)", m.screen, m.cfg.Mode)
	}
}

func TestHistoryOpensAndReturns(t *testing.T) {
	m := newTestModel(t)
	press(m, "esc")
	goToRow(m, rowOf(t, m, "history"))
	press(m, "enter")
	if m.screen != scrHistory {
		t.Fatal("enter on history should open it")
	}
	m.View()
	press(m, "enter", "esc") // no tests: enter does nothing, esc returns
	if m.screen != scrMenu {
		t.Fatalf("esc in history should return to the menu, got %v", m.screen)
	}
}
