package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jinchiwei/sanda/internal/analysis"
)

func (m *Model) viewResult() string {
	st := m.stats
	tw := m.textWidth()
	var out []string

	// Headline: big wpm and accuracy, like Monkeytype's left column.
	wpm := m.big(fmt.Sprintf("%.0f", st.WPM), m.th.Main)
	acc := m.big(fmt.Sprintf("%.0f%%", st.Acc), m.warm(m.th.Main))
	out = append(out, m.fg(m.hot(m.th.Sub)).Render("wpm")+m.bgSpaces(lipgloss.Width(wpm[0])+3)+m.fg(m.hot(m.th.Sub)).Render("acc"))
	for i := range wpm {
		out = append(out, wpm[i]+m.bgSpaces(6)+acc[i])
	}
	out = append(out, m.bgSpaces(1))

	stat := func(label, v string) string { return m.sub(label+" ") + m.text(v) }
	statC := func(label, v, hex string) string { return m.sub(label+" ") + m.fg(hex).Render(v) }
	chars := m.text(fmt.Sprint(st.Correct)) + m.sub("/") + m.fg(m.th.Error).Render(fmt.Sprint(st.Incorrect)) + m.sub("/") +
		m.fg(m.th.ErrorExtra).Render(fmt.Sprint(st.Extra)) + m.sub("/") + m.sub(fmt.Sprint(st.Missed))
	secs := st.Duration.Seconds()
	line := strings.Join([]string{
		statC("raw", fmt.Sprintf("%.0f", st.Raw), m.warm(m.th.Text)),
		statC("consistency", fmt.Sprintf("%.0f%%", st.Consistency), m.hot(m.th.Text)),
		m.sub("characters ") + chars,
		stat("time", fmt.Sprintf("%.0fs", secs)),
	}, m.bgSpaces(3))
	out = append(out, line)
	out = append(out, m.sub(m.testLabel()))
	if m.activity == ActTest && m.pb {
		if m.prevPB > 0 {
			out = append(out, m.fg(m.warm(m.th.Caret)).Render(fmt.Sprintf("★ new personal best (was %.0f)", m.prevPB)))
		} else {
			out = append(out, m.sub("first result for this test type"))
		}
	}
	out = append(out, m.bgSpaces(1))
	out = append(out, m.chart(st.PerSecond, tw, 7)...)

	// Quote source, Monkeytype style.
	if m.quote != nil {
		out = append(out, m.bgSpaces(1), m.fg(m.hot(m.th.Sub)).Render("source"))
		for _, l := range wrapText(m.quote.Source, tw) {
			out = append(out, m.text(l))
		}
	}

	switch m.activity {
	case ActTrain:
		out = append(out, m.bgSpaces(1))
		out = append(out, m.trainOutcome(tw)...)
	case ActPace:
		out = append(out, m.bgSpaces(1), m.main(m.paceNote))
		next := "next: control round (no pacer, aim for 97%+ accuracy)"
		if m.phase == phaseControl {
			next = fmt.Sprintf("next: burst at %.0f wpm", m.target)
		}
		out = append(out, m.sub(next))
	}

	// What cost you: the top three estimated losses.
	if len(m.losses) > 0 {
		out = append(out, m.bgSpaces(1), m.fg(m.hot(m.th.Sub)).Render("what cost you"))
		for i, l := range m.losses {
			if i == 3 || l.Seconds < 0.15 {
				break
			}
			out = append(out, m.fg(m.warm(m.th.Error)).Render(fmt.Sprintf("−%.1fs", l.Seconds))+m.bgSpaces(2)+m.text(l.Label))
		}
	}

	out = append(out, m.bgSpaces(1))
	next := " next test   "
	switch m.activity {
	case ActTrain:
		next = " next drill   "
	case ActPace:
		next = " next round   "
	}
	out = append(out, m.keycap("tab", m.warm(m.th.Text))+m.sub(next)+m.keycap("esc", m.hot(m.th.Text))+m.sub(" menu   ")+m.keycap("q", m.th.Text)+m.sub(" quit"))
	if m.status != "" {
		out = append(out, m.fg(m.th.Error).Render(m.status))
	}
	return m.block(out)
}

func (m *Model) testLabel() string {
	switch m.activity {
	case ActTrain:
		return "train · drill (not counted in your wpm history)"
	case ActPace:
		if m.phase == phaseBurst {
			return "pace · burst (not counted in your wpm history)"
		}
		return "pace · control (not counted in your wpm history)"
	}
	var parts []string
	switch m.cfg.Mode {
	case "time":
		parts = append(parts, fmt.Sprintf("time %d", m.cfg.Time))
	case "words":
		parts = append(parts, fmt.Sprintf("words %d", m.cfg.Words))
	case "quote":
		parts = append(parts, "quote "+m.cfg.QuoteLength)
	}
	if m.cfg.Punctuation {
		parts = append(parts, "punctuation")
	}
	if m.cfg.Numbers {
		parts = append(parts, "numbers")
	}
	return "test type  " + strings.Join(parts, " · ")
}

func (m *Model) trainOutcome(tw int) []string {
	out := []string{m.sub(fmt.Sprintf("%-8s %9s %9s %9s   %s", "focus", "this", "before", "baseline", "result"))}
	ms := func(d time.Duration) string {
		if d == 0 {
			return "–"
		}
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	for _, o := range m.outcome {
		var res string
		switch {
		case o.Samples < 2:
			res = m.sub("not enough clean samples")
		case o.Passed:
			res = m.main(fmt.Sprintf("✓ fast enough, next review in %s", days(time.Until(o.NextDue))))
		default:
			res = m.fg(m.th.Error).Render("✗ still slow, keep drilling")
		}
		row := fmt.Sprintf("%-8s %9s %9s %9s   ", analysis.Show(o.Unit), ms(o.Round), ms(o.Before), ms(o.Baseline))
		out = append(out, m.text(row)+res)
	}
	return out
}

func days(d time.Duration) string {
	n := int(d.Hours()/24 + 0.5)
	if n <= 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", n)
}

// wrapText wraps plain text to width.
func wrapText(s string, width int) []string {
	var lines []string
	line := ""
	for _, w := range strings.Fields(s) {
		if line != "" && len([]rune(line))+1+len([]rune(w)) > width {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += w
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
