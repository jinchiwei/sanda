package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jinchiwei/sanda/internal/analysis"
	"github.com/jinchiwei/sanda/internal/data"
	"github.com/jinchiwei/sanda/internal/engine"
	"github.com/jinchiwei/sanda/internal/store"
	"github.com/jinchiwei/sanda/internal/train"
)

// resultView is everything the results screen shows, for the test just finished or one from history.
type resultView struct {
	stats  engine.Stats
	losses []analysis.Loss
	quote  *data.Quote
	label  string
	notes  []string // already rendered lines: personal best, train outcome, pace verdict
	footer string
}

func (m *Model) viewResult() string {
	rv := resultView{stats: m.stats, losses: m.losses, quote: m.quote, label: m.testLabel()}
	tw := m.textWidth()
	if m.activity == ActTest && m.pb {
		if m.prevPB > 0 {
			rv.notes = append(rv.notes, m.fg(m.th.Caret).Render(fmt.Sprintf("★ new personal best (was %.0f)", m.prevPB)))
		} else {
			rv.notes = append(rv.notes, m.sub("first result for this test type"))
		}
	}
	if m.activity == ActTest && m.quote != nil {
		rv.notes = append(rv.notes, m.quoteRecordLine(m.quoteRec, m.stats.WPM, len(m.quoteRec)+1, true))
	}
	var after []string
	if m.activity == ActTest && m.bests != nil {
		after = append(after, m.bgSpaces(1))
		after = append(after, m.bestsTable()...)
	}
	switch m.activity {
	case ActTrain:
		after = append(after, m.bgSpaces(1))
		after = append(after, m.trainOutcome(tw)...)
	case ActPace:
		after = append(after, m.bgSpaces(1), m.main(m.paceNote))
		next := "next: control round (no pacer, aim for 97%+ accuracy)"
		if m.phase == phaseControl {
			next = fmt.Sprintf("next: burst at %.0f wpm", m.target)
		}
		after = append(after, m.sub(next))
	}
	next := " next test   "
	switch m.activity {
	case ActTrain:
		next = " next drill   "
	case ActPace:
		next = " next round   "
	}
	rv.footer = m.keycap("tab", m.th.Text) + m.sub(next) + m.keycap("r", m.th.Text) + m.sub(" retake   ") + m.keycap("esc", m.th.Text) + m.sub(" menu   ") + m.keycap("q", m.th.Text) + m.sub(" quit")
	return m.renderResult(rv, after)
}

// renderResult draws a results screen; after goes between the chart/source and "what cost you".
func (m *Model) renderResult(rv resultView, after []string) string {
	st := rv.stats
	tw := m.textWidth()
	var out []string

	// Headline: big wpm and accuracy, like Monkeytype's left column.
	wpm := m.big(fmt.Sprintf("%.0f", st.WPM), m.th.Caret)
	acc := m.big(fmt.Sprintf("%.0f%%", st.Acc), m.th.Main)
	out = append(out, m.fg(m.th.Sub).Render("wpm")+m.bgSpaces(lipgloss.Width(wpm[0])+3)+m.fg(m.th.Sub).Render("acc"))
	for i := range wpm {
		out = append(out, wpm[i]+m.bgSpaces(6)+acc[i])
	}
	out = append(out, m.bgSpaces(1))

	stat := func(label, v string) string { return m.sub(label+" ") + m.text(v) }
	statC := func(label, v, hex string) string { return m.sub(label+" ") + m.fg(hex).Render(v) }
	chars := m.fg(m.th.Typed).Render(fmt.Sprint(st.Correct)) + m.sub("/") + m.fg(m.th.Error).Render(fmt.Sprint(st.Incorrect)) + m.sub("/") +
		m.fg(m.th.ErrorExtra).Render(fmt.Sprint(st.Extra)) + m.sub("/") + m.sub(fmt.Sprint(st.Missed))
	line := strings.Join([]string{
		statC("raw", fmt.Sprintf("%.0f", st.Raw), m.th.Typed),
		statC("consistency", fmt.Sprintf("%.0f%%", st.Consistency), m.th.Typed),
		m.sub("characters ") + chars,
		stat("time", fmt.Sprintf("%.0fs", st.Duration.Seconds())),
	}, m.bgSpaces(3))
	out = append(out, line, m.sub(rv.label))
	out = append(out, rv.notes...)
	out = append(out, m.bgSpaces(1))
	out = append(out, m.chart(st.PerSecond, tw, 7)...)

	// Quote source, Monkeytype style.
	if rv.quote != nil {
		out = append(out, m.bgSpaces(1), m.fg(m.th.Sub).Render("source"))
		for _, l := range wrapText(rv.quote.Source, tw) {
			out = append(out, m.text(l))
		}
	}
	out = append(out, after...)

	// What cost you: the top three estimated losses.
	if len(rv.losses) > 0 && rv.losses[0].Seconds >= 0.15 {
		out = append(out, m.bgSpaces(1), m.fg(m.th.Sub).Render("what cost you"))
		for i, l := range rv.losses {
			if i == 3 || l.Seconds < 0.15 {
				break
			}
			out = append(out, m.fg(m.th.Error).Render(fmt.Sprintf("−%.1fs", l.Seconds))+m.bgSpaces(2)+m.text(l.Label))
		}
	}

	out = append(out, m.bgSpaces(1), rv.footer)
	if m.status != "" {
		out = append(out, m.fg(m.th.Error).Render(m.status))
	}
	return m.block(out)
}

// bestsTable is Monkeytype's personal-best grid: wpm and accuracy for every time and words length,
// under the current punctuation/numbers settings. The test just finished is highlighted, starred if new.
func (m *Model) bestsTable() []string {
	const w = 7
	cur := map[string]string{"time": fmt.Sprint(m.cfg.Time), "words": fmt.Sprint(m.cfg.Words)}[m.cfg.Mode]
	title := "personal bests"
	if m.cfg.Punctuation {
		title += " · punctuation"
	}
	if m.cfg.Numbers {
		title += " · numbers"
	}
	head, wpm, acc := []string{m.sub(fmt.Sprintf("%-15s", title))}, []string{m.bgSpaces(15)}, []string{m.bgSpaces(15)}
	for gi, g := range []struct {
		mode, unit string
		vals       []string
	}{{"time", "s", []string{"15", "30", "60", "120"}}, {"words", "w", []string{"10", "25", "50", "100"}}} {
		if gi > 0 {
			head, wpm, acc = append(head, m.sub(" │ ")), append(wpm, m.sub(" │ ")), append(acc, m.sub(" │ "))
		}
		for _, v := range g.vals {
			here := g.mode == m.cfg.Mode && v == cur
			label := v + g.unit
			if here && m.pb {
				label = "★" + label
			}
			col := m.th.Sub
			if here {
				col = m.th.Caret
			}
			head = append(head, m.fg(col).Render(fmt.Sprintf("%*s", w, label)))
			if t, ok := m.bests[g.mode][v]; ok {
				wc := m.th.Text
				if here {
					wc = m.th.Caret
				}
				wpm = append(wpm, m.fg(wc).Render(fmt.Sprintf("%*.0f", w, t.WPM)))
				acc = append(acc, m.sub(fmt.Sprintf("%*s", w, fmt.Sprintf("%.0f%%", t.Acc))))
			} else {
				wpm = append(wpm, m.sub(fmt.Sprintf("%*s", w, "–")))
				acc = append(acc, m.bgSpaces(w))
			}
		}
	}
	return []string{strings.Join(head, ""), strings.Join(wpm, ""), strings.Join(acc, "")}
}

// quoteRecordLine summarises your record on one quote. earlier are the official attempts before this one;
// attempt is this attempt's number; live marks a result that was just typed (vs one reopened from history).
func (m *Model) quoteRecordLine(earlier []store.Test, wpm float64, attempt int, live bool) string {
	best := 0.0
	for _, t := range earlier {
		best = math.Max(best, t.WPM)
	}
	label := m.sub("this quote  ")
	switch {
	case len(earlier) == 0:
		return label + m.fg(m.th.Typed).Render("first attempt")
	case wpm > best:
		return label + m.sub(fmt.Sprintf("attempt %d · ", attempt)) +
			m.fg(m.th.Caret).Render(fmt.Sprintf("★ new best on this quote (was %.0f)", best))
	}
	verb := "best"
	if !live {
		verb = "best before this"
	}
	return label + m.sub(fmt.Sprintf("attempt %d · %s ", attempt, verb)) + m.fg(m.th.Typed).Render(fmt.Sprintf("%.0f", best)) +
		m.sub(fmt.Sprintf(" (%+.0f)", wpm-best))
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
	out := []string{m.sub("training for " + m.trainFormat().Name() + " tests"), m.sub(fmt.Sprintf("%-8s %9s %9s %9s   %s", "focus", "this", "before", "par", "result"))}
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
			res = m.fg(m.th.Error).Render(fmt.Sprintf("✗ still slow, back in %d min", int(train.Retry.Minutes())))
		}
		row := fmt.Sprintf("%-8s %9s %9s %9s   ", analysis.Show(o.Unit), ms(o.Round), ms(o.Before), ms(o.Target))
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

// until is a short "in …" for review times, which run from minutes (a miss) to weeks (passes).
func until(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d min", max(1, int(d.Minutes()+0.5)))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h", int(d.Hours()+0.5))
	}
	return days(d)
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
