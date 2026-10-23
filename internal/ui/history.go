package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jinchiwei/sanda/internal/analysis"
	"github.com/jinchiwei/sanda/internal/data"
	"github.com/jinchiwei/sanda/internal/engine"
	"github.com/jinchiwei/sanda/internal/store"
	"github.com/jinchiwei/sanda/internal/train"
)

// History lists finished tests, newest first; enter reopens a test's full results from its saved keystrokes.

func (m *Model) openHistory(from screen) {
	m.subBack = from
	m.histIdx = 0
	m.histDetail = false
	m.loadHistory()
	m.screen = scrHistory
}

func (m *Model) loadHistory() {
	kinds := []string{store.KindOfficial}
	if m.histAll {
		kinds = nil
	}
	tests, err := m.db.Tests(time.Time{}, kinds...)
	if err != nil {
		m.status = "could not read history: " + err.Error()
	}
	m.histTests = m.histTests[:0]
	for i := len(tests) - 1; i >= 0; i-- {
		m.histTests = append(m.histTests, tests[i])
	}
	m.histIdx = min(m.histIdx, max(len(m.histTests)-1, 0))
}

func histLabel(t store.Test) string {
	l := t.Mode + " " + t.ModeValue
	switch t.Kind {
	case store.KindTrain:
		l = "train drill" // rounds from before train had formats
		for _, f := range []train.Format{train.Plain, train.Prose} {
			if t.Mode == f.DrillMode() {
				l = "train " + f.Name()
			}
		}
	case store.KindPace:
		l = "pace " + t.Mode
	}
	if t.Punctuation {
		l += " @"
	}
	if t.Numbers {
		l += " #"
	}
	return l
}

func (m *Model) keyHistory(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.histDetail {
		switch k.String() {
		case "esc", "enter", "backspace":
			m.histDetail = false
		case "q":
			return m, tea.Quit
		}
		return m, nil
	}
	n := len(m.histTests)
	switch k.String() {
	case "down", "j":
		m.histIdx = min(m.histIdx+1, max(n-1, 0))
	case "up", "k":
		m.histIdx = max(m.histIdx-1, 0)
	case "pgdown", "ctrl+d":
		m.histIdx = min(m.histIdx+10, max(n-1, 0))
	case "pgup", "ctrl+u":
		m.histIdx = max(m.histIdx-10, 0)
	case "g", "home":
		m.histIdx = 0
	case "G", "end":
		m.histIdx = max(n-1, 0)
	case "a":
		m.histAll = !m.histAll
		m.loadHistory()
	case "enter":
		if n > 0 {
			m.histDetail = true
		}
	case "esc":
		m.screen = m.subBack
		if m.screen == scrMenu {
			m.buildMenu()
		}
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) viewHistory() string {
	if m.histDetail && m.histIdx < len(m.histTests) {
		return m.historyDetail(m.histTests[m.histIdx])
	}
	filter := "official tests"
	if m.histAll {
		filter = "all rounds (incl. train and pace)"
	}
	out := []string{m.main("history") + m.sub("  "+filter), m.bgSpaces(1)}
	out = append(out, m.fg(m.th.Sub).Render(fmt.Sprintf("  %-17s %-16s %5s %6s %5s %6s %6s", "when", "test", "wpm", "acc", "raw", "cons", "time")))
	if len(m.histTests) == 0 {
		out = append(out, m.sub("  no finished tests yet"))
	}
	rows := max(m.h-14, 5)
	start := min(max(0, m.histIdx-rows/2), max(len(m.histTests)-rows, 0))
	for i := start; i < len(m.histTests) && i < start+rows; i++ {
		t := m.histTests[i]
		line := fmt.Sprintf("  %-17s %-16s %5.0f %5.0f%% %5.0f %5.0f%% %5.0fs", t.StartedAt.Local().Format("Jan 02 15:04"),
			histLabel(t), t.WPM, t.Acc, t.Raw, t.Cons, float64(t.DurationMs)/1000)
		st := m.fg(m.th.Text)
		if i == m.histIdx {
			st = st.Background(lipgloss.Color(m.th.SubAlt)).Foreground(lipgloss.Color(m.th.Caret))
		}
		out = append(out, st.Render(line))
	}
	out = append(out, m.bgSpaces(1), m.sub(fmt.Sprintf("%d · j/k move · enter details · a official/all · esc back", len(m.histTests))))
	return m.block(out)
}

// historyDetail rebuilds a saved test's results screen from its stored keystrokes.
func (m *Model) historyDetail(t store.Test) string {
	events, _ := m.db.TestEventsFor(t.ID)
	dur := time.Duration(t.DurationMs) * time.Millisecond
	rv := resultView{
		stats: engine.Stats{WPM: t.WPM, Raw: t.Raw, Acc: t.Acc, Consistency: t.Cons, Duration: dur,
			Correct: t.Correct, Incorrect: t.Incorrect, Extra: t.Extra, Missed: t.Missed,
			PerSecond: engine.PerSecond(events, dur)},
		losses: analysis.Breakdown(events, strings.Fields(t.Text), dur),
		label:  t.StartedAt.Local().Format("Mon Jan 2 2006, 15:04") + "  ·  " + histLabel(t),
		footer: m.keycap("esc", m.th.Text) + m.sub(" back to history   ") + m.keycap("q", m.th.Text) + m.sub(" quit"),
	}
	if t.QuoteID > 0 {
		if q, ok := data.QuoteByID(t.QuoteID); ok {
			rv.quote = &q
		}
		// The quote's record as it stood when this attempt was typed.
		if all, err := m.db.QuoteAttempts(t.QuoteID); err == nil && t.Kind == store.KindOfficial {
			var earlier []store.Test
			for _, a := range all {
				if a.ID == t.ID {
					break
				}
				earlier = append(earlier, a)
			}
			rv.notes = append(rv.notes, m.quoteRecordLine(earlier, t.WPM, len(earlier)+1, false))
		}
	}
	return m.renderResult(rv, nil)
}
