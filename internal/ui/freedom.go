package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// freeWriting holds only the current text and clock in memory. It never creates
// engine events or a scored session, so it cannot feed history or training.
type freeWriting struct {
	text    []rune
	started time.Time
}

func (f *freeWriting) wpm(now time.Time) float64 {
	if f.started.IsZero() || now.Sub(f.started) < time.Second {
		return 0
	}
	return float64(len(f.text)) / 5 / now.Sub(f.started).Minutes()
}

func (m *Model) startFreedom() {
	m.activity = ActFreedom
	m.free = freeWriting{}
	m.sess = nil
	m.words = nil
	m.quote = nil
	m.status = ""
	m.screen = scrTyping
}

func (m *Model) typingStarted() bool {
	if m.activity == ActFreedom {
		return !m.free.started.IsZero()
	}
	return m.sess != nil && m.sess.Started() && !m.sess.Finished()
}

func (m *Model) keyFreedom(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if k.Paste {
		return m, nil
	}
	switch k.String() {
	case "tab":
		m.startFreedom()
	case "esc":
		m.openMenu(scrTyping)
	case "backspace", "ctrl+h":
		if len(m.free.text) > 0 {
			m.free.text = m.free.text[:len(m.free.text)-1]
		}
	case "ctrl+w", "alt+backspace", "ctrl+backspace":
		for len(m.free.text) > 0 && unicode.IsSpace(m.free.text[len(m.free.text)-1]) {
			m.free.text = m.free.text[:len(m.free.text)-1]
		}
		for len(m.free.text) > 0 && !unicode.IsSpace(m.free.text[len(m.free.text)-1]) {
			m.free.text = m.free.text[:len(m.free.text)-1]
		}
	default:
		var added []rune
		if k.Type == tea.KeyEnter {
			added = []rune{'\n'}
		} else if k.Type == tea.KeySpace {
			added = []rune{' '}
		} else if k.Type == tea.KeyRunes && !k.Alt {
			for _, r := range k.Runes {
				if unicode.IsPrint(r) {
					added = append(added, r)
				}
			}
		}
		if len(added) > 0 {
			if m.free.started.IsZero() {
				m.free.started = time.Now()
			}
			m.free.text = append(m.free.text, added...)
		}
	}
	return m, nil
}

func (m *Model) viewFreedom() string {
	width := m.textWidth()
	header := m.main("freedom") + m.bgSpaces(3) + m.fg(m.th.Caret).Render(fmt.Sprintf("%.0f", m.free.wpm(time.Now()))) + m.sub(" wpm")
	lines := []string{}
	var line strings.Builder
	col := 0
	for _, r := range m.free.text {
		if r == '\n' {
			lines = append(lines, line.String())
			line.Reset()
			col = 0
			continue
		}
		rw := lipgloss.Width(string(r))
		if col+rw > width {
			lines = append(lines, line.String())
			line.Reset()
			col = 0
		}
		line.WriteRune(r)
		col += rw
	}
	if col >= width {
		lines = append(lines, line.String())
		line.Reset()
	}
	lines = append(lines, line.String())
	// Follow the end of the writing, keeping the screen height stable.
	const rows = 6
	if len(lines) > rows {
		lines = lines[len(lines)-rows:]
	}
	out := []string{m.pad(header, width), m.bgSpaces(width)}
	for i, l := range lines {
		rendered := m.fg(m.th.Typed).Render(l)
		if i == len(lines)-1 {
			rendered += m.fg(m.th.OnCaret()).Background(lipgloss.Color(m.th.Caret)).Render(" ")
		}
		out = append(out, m.pad(rendered, width))
	}
	for len(out) < rows+2 {
		out = append(out, m.bgSpaces(width))
	}
	out = append(out, m.bgSpaces(width), m.sub("nothing saved · tab clear · esc menu · ctrl+c quit"))
	return m.block(out)
}
