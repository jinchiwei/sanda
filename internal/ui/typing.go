package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jinchiwei/sanda/internal/analysis"
)

// cell is one character on the typing screen.
type cell struct {
	r        rune
	fg       string
	bg       string
	under    bool
	mistyped rune // actual input, displayed below an incorrect expected character
}

// typingRows wraps the cells that will actually be drawn, including extra
// input. Long words continue on another row instead of being clipped. The
// current row follows the caret, including a caret just beyond a full line.
func (m *Model) typingRows(width, ghost int) ([][]cell, int) {
	s := m.sess
	rows := [][]cell{nil}
	col, offset, current := 0, 0, 0
	newRow := func() {
		rows = append(rows, nil)
		col = 0
	}
	for wi, word := range s.Words {
		cells := m.wordCells(wi, offset, ghost)
		offset += len(word) + 1
		caret := -1
		if wi == s.Cur && !s.Finished() {
			caret = len(s.Input[wi])
		}
		// Ignore the separator when deciding whether a word fits, unless it
		// holds the caret. Ordinary spaces may fall outside the right edge.
		wordWidth := 0
		for j, c := range cells {
			if j < len(cells)-1 || j == caret {
				wordWidth += lipgloss.Width(string(c.r))
			}
		}
		if col > 0 && col+wordWidth > width {
			newRow()
		}
		if wi == s.Cur {
			current = len(rows) - 1
		}
		for j, c := range cells {
			cw := lipgloss.Width(string(c.r))
			if col+cw > width {
				if j == len(cells)-1 && j != caret {
					break
				}
				newRow()
			}
			rows[len(rows)-1] = append(rows[len(rows)-1], c)
			col += cw
			if j == caret {
				current = len(rows) - 1
			}
		}
	}
	return rows, current
}

func (m *Model) viewTyping() string {
	s := m.sess
	tw := m.textWidth()
	now := time.Now()

	ghost := -1
	if m.activity == ActPace && m.phase == phaseBurst && s.Started() {
		ghost = int(m.target * 5 / 60 * s.Elapsed(now).Seconds())
	}
	rows, current := m.typingRows(tw, ghost)
	first := max(current-1, 0)
	var lines []string
	for li := first; li < first+3 && li < len(rows); li++ {
		lines = append(lines, m.renderCells(rows[li], tw), m.renderMistypes(rows[li], tw))
	}
	for len(lines) < 6 {
		lines = append(lines, m.bgSpaces(tw))
	}

	// The language sits above the words before the test starts, like Monkeytype.
	lang := m.bgSpaces(tw)
	if !s.Started() {
		lang = m.center(m.sub("english"), tw)
	}
	out := []string{m.pad(m.typingHeader(now), tw), lang}
	out = append(out, lines...)
	out = append(out, m.bgSpaces(tw), m.pad(m.typingFooter(), tw))
	if m.status != "" {
		out = append(out, m.pad(m.fg(m.th.Error).Render(m.status), tw))
	}
	return m.block(out)
}

// wordCells styles one word plus its trailing space, Monkeytype style.
func (m *Model) wordCells(wi, off, ghost int) []cell {
	s := m.sess
	word := s.Words[wi]
	var in []rune
	if wi < len(s.Input) {
		in = s.Input[wi]
	}
	submitted := wi < s.Cur
	wrong := submitted && !s.WordCorrect(wi)
	var cells []cell
	for j, r := range word {
		c := cell{r: r, fg: m.th.Untyped, bg: m.th.BgHex(), under: wrong}
		if j < len(in) {
			if in[j] == r {
				c.fg = m.th.Typed
			} else {
				c.fg = m.th.Error
				c.mistyped = in[j]
			}
		} else if submitted && j == len(in) {
			// Space submitted this word before its expected end.
			c.fg = m.th.Error
			c.mistyped = '␣'
		}
		if off+j == ghost {
			c.bg = m.th.SubAlt
			c.fg = m.th.Main
		}
		cells = append(cells, c)
	}
	for j := len(word); j < len(in); j++ {
		cells = append(cells, cell{r: in[j], fg: m.th.ErrorExtra, bg: m.th.BgHex(), under: wrong})
	}
	// Caret: a block in the caret colour on the next character to type.
	if wi == s.Cur && !s.Finished() {
		pos := len(in)
		if pos < len(cells) {
			cells[pos].bg, cells[pos].fg = m.th.Caret, m.th.OnCaret()
		} else {
			cells = append(cells, cell{r: ' ', fg: m.th.OnCaret(), bg: m.th.Caret})
			return cells
		}
	}
	sp := cell{r: ' ', fg: m.th.Untyped, bg: m.th.BgHex()}
	if off+len(word) == ghost {
		sp.bg = m.th.SubAlt
	}
	return append(cells, sp)
}

// renderMistypes keeps actual incorrect letters aligned beneath the prompt.
// Empty hint rows are retained so correcting a mistake never moves the text.
func (m *Model) renderMistypes(cells []cell, width int) string {
	hints := make([]cell, min(len(cells), width))
	for i := range hints {
		r := cells[i].mistyped
		if r == 0 {
			r = ' '
		}
		hints[i] = cell{r: r, fg: m.th.Error, bg: m.th.BgHex()}
	}
	return m.renderCells(hints, width)
}

// renderCells groups runs of identical style so lipgloss isn't called per character.
func (m *Model) renderCells(cells []cell, width int) string {
	columns := 0
	for i, c := range cells {
		columns += lipgloss.Width(string(c.r))
		if columns > width {
			cells = cells[:i]
			break
		}
	}
	var sb strings.Builder
	for i := 0; i < len(cells); {
		j := i
		var run []rune
		for j < len(cells) && cells[j].fg == cells[i].fg && cells[j].bg == cells[i].bg && cells[j].under == cells[i].under {
			run = append(run, cells[j].r)
			j++
		}
		st := bgStyle(lipgloss.NewStyle().Foreground(lipgloss.Color(cells[i].fg)), cells[i].bg)
		if cells[i].under {
			st = st.Underline(true)
		}
		sb.WriteString(st.Render(string(run)))
		i = j
	}
	return m.pad(sb.String(), width)
}

func (m *Model) typingHeader(now time.Time) string {
	s := m.sess
	var left string
	switch {
	case s.TimeLimit > 0:
		left = m.fg(m.th.Main).Render(fmt.Sprintf("%d", int((s.TimeLimit-s.Elapsed(now)).Seconds()+0.999)))
	default:
		left = m.fg(m.th.Main).Render(fmt.Sprintf("%d/%d", s.Cur, len(s.Words)))
	}
	if s.Started() && m.cfg.LiveWPM {
		live := "–"
		if w := s.LiveWPM(now); w > 0 {
			live = fmt.Sprintf("%.0f", w)
		}
		left += m.bgSpaces(3) + m.fg(m.th.Caret).Render(live) + m.sub(" wpm")
	}
	if !s.Started() {
		left = m.configLine()
	}
	switch m.activity {
	case ActTrain:
		var units []string
		for _, f := range m.focus {
			u := analysis.Show(f.Unit)
			if f.Probe {
				u += "?"
			}
			units = append(units, u)
		}
		left += m.fg(m.th.Sub).Render("   train · ") + m.fg(m.th.Main).Render(strings.Join(units, "  "))
	case ActPace:
		if m.phase == phaseBurst {
			left += m.fg(m.th.Sub).Render("   burst · ") + m.sub("pacer ") + m.fg(m.th.Main).Render(fmt.Sprintf("%.0f", m.target)) + m.sub(" wpm")
			if s.Started() {
				mine := float64(s.CaretOffset()) / 5 / s.Elapsed(now).Minutes()
				if s.Elapsed(now) > time.Second {
					diff := mine - m.target
					col := m.th.Main
					if diff < 0 {
						col = m.th.Error
					}
					left += m.fg(col).Render(fmt.Sprintf("  %+.0f", diff))
				}
			}
		} else {
			left += m.fg(m.th.Sub).Render("   control · ") + m.sub("type clean, no pacer")
		}
	}
	return left
}

// configLine shows the current test settings before the test starts.
func (m *Model) configLine() string {
	if m.activity != ActTest {
		return ""
	}
	var mode string
	switch m.cfg.Mode {
	case "time":
		mode = fmt.Sprintf("time %d", m.cfg.Time)
	case "words":
		mode = fmt.Sprintf("words %d", m.cfg.Words)
	case "quote":
		mode = "quote " + m.cfg.QuoteLength
	}
	sep := m.sub(" · ")
	punct, nums := m.sub("@ punctuation"), m.sub("# numbers")
	if m.cfg.Punctuation {
		punct = m.fg(m.th.Main).Render("@ punctuation")
	}
	if m.cfg.Numbers {
		nums = m.fg(m.th.Main).Render("# numbers")
	}
	return punct + sep + nums + sep + m.main(mode)
}

func (m *Model) typingFooter() string {
	return m.keycap("tab", m.th.Text) + m.sub(" restart   ") + m.keycap("esc", m.th.Text) + m.sub(" menu")
}
