package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jinchiwei/sanda/internal/config"
)

// Freedom is free writing: there must be no prompt or correctness scoring.
func TestFreedomFreeWritingAndLiveWPM(t *testing.T) {
	base := newTestModel(t)
	cfg := config.Default()
	cfg.LiveWPM = false
	m := New(cfg, base.db, "freedom")
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.sess != nil {
		t.Fatal("freedom must not create a scored typing test")
	}
	press(m, "my own words", "enter", "another line")
	m.Update(tickMsg(time.Now().Add(24 * time.Hour)))
	view := m.View()
	if !strings.Contains(view, "my own words") || !strings.Contains(view, "another line") || !strings.Contains(view, "wpm") {
		t.Fatalf("free writing and WPM must be visible: %q", view)
	}
	if strings.Contains(view, "accuracy") || strings.Contains(view, "english") {
		t.Fatal("freedom must not show a test prompt or score")
	}
	if m.screen != scrTyping {
		t.Fatal("freedom must not expire")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if !strings.Contains(m.View(), "another lin") {
		t.Fatal("backspace must edit free text")
	}
}

// The normal completion/save path must never persist free-writing text or events.
func TestFreedomNeverSavesAndCanRestartOrLeave(t *testing.T) {
	m := newTestModel(t)
	press(m, "esc")
	goToRow(m, rowOf(t, m, "freedom"))
	press(m, "enter", "private writing")
	m.checkDone()
	press(m, "tab")
	if strings.Contains(m.View(), "private writing") || m.sess != nil {
		t.Fatal("tab must clear writing and stay in freedom")
	}
	press(m, "more private writing", "esc", "esc")
	if m.sess != nil || strings.Contains(m.View(), "more private writing") {
		t.Fatal("returning from menu must restart freedom")
	}
	press(m, "esc")
	goToRow(m, rowOf(t, m, "15"))
	press(m, "enter")
	if m.sess == nil || m.sess.TimeLimit != 15*time.Second {
		t.Fatal("choosing a test must leave freedom")
	}
	tests, err := m.db.Tests(time.Time{})
	if err != nil || len(tests) != 0 {
		t.Fatalf("freedom saved tests: %v %v", tests, err)
	}
	events, err := m.db.Events(time.Time{})
	if err != nil || len(events) != 0 {
		t.Fatalf("freedom saved keystrokes: %v %v", events, err)
	}
	schedule, err := m.db.Schedule()
	if err != nil || len(schedule) != 0 {
		t.Fatalf("freedom changed training schedule: %v %v", schedule, err)
	}
}

// A raw keypress counter would reward corrections; freedom reports net writing speed.
func TestFreedomWPMCountsCurrentTextAndEditing(t *testing.T) {
	base := newTestModel(t)
	m := New(config.Default(), base.db, "freedom")
	press(m, "hello world") // 11 characters, not two five-character words
	m.free.started = time.Unix(1000, 0)
	now := m.free.started.Add(30 * time.Second)
	if got := m.free.wpm(now); got != 4.4 {
		t.Fatalf("11 characters over 30 seconds = 4.4 WPM, got %v", got)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	if string(m.free.text) != "hello " {
		t.Fatalf("word deletion left %q", string(m.free.text))
	}
	if got := m.free.wpm(now); got != 2.4 {
		t.Fatalf("6 remaining characters = 2.4 WPM, got %v", got)
	}
	press(m, "tab")
	if got := m.free.wpm(now); got != 0 {
		t.Fatalf("clearing must reset WPM, got %v", got)
	}
}
