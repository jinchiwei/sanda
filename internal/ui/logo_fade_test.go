package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// A binary hide on the first key or a stale full-brightness logo cache breaks this.
func TestLogoDimsBeforeDisappearingWithoutBlockingTyping(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })
	for _, freedom := range []bool{false, true} {
		m := newTestModel(t)
		m.cfg.KeepLogo = false
		if freedom {
			m.startFreedom()
		}
		m.Update(tea.WindowSizeMsg{Width: 140, Height: 60})
		full := strings.Join(m.logoLines(logoPixels), "\n")
		if !strings.Contains(m.View(), "▀") {
			t.Fatal("idle logo missing")
		}
		start := time.Now()
		press(m, "a")
		if !strings.Contains(m.View(), "▀") {
			t.Fatal("logo disappeared immediately instead of fading")
		}
		m.Update(tickMsg(start.Add(75 * time.Millisecond)))
		half := strings.Join(m.logoLines(logoPixels), "\n")
		if half == full {
			t.Fatal("logo did not dim during fade")
		}
		press(m, "b")
		if freedom && string(m.free.text) != "ab" {
			t.Fatal("fade blocked free writing")
		}
		if !freedom && len(m.sess.Events) != 2 {
			t.Fatal("fade blocked typing events")
		}
		m.Update(tickMsg(start.Add(250 * time.Millisecond)))
		if strings.Contains(m.View(), "▀") {
			t.Fatal("logo stayed visible after quick fade")
		}
		press(m, "tab")
		if !strings.Contains(m.View(), "▀") {
			t.Fatal("restart did not restore logo")
		}
	}
}

func TestKeepLogoNeverDimsOnFocus(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })
	m := newTestModel(t)
	m.cfg.KeepLogo = true
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 60})
	full := strings.Join(m.logoLines(logoPixels), "\n")
	press(m, "a")
	m.Update(tickMsg(time.Now().Add(time.Second)))
	if !strings.Contains(m.View(), "▀") || strings.Join(m.logoLines(logoPixels), "\n") != full {
		t.Fatal("keep-logo dimmed or hid logo")
	}
}
