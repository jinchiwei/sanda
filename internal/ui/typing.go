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
	r     rune
	fg    string
	bg    string
	under bool
}

// wrap assigns each word to a line given the text width; returns the first word index of each line.
func wrap(words [][]rune, width int) []int {
	starts := []int{0}
	col := 0
	for i, w := range words {
		l := len(w)
		if col > 0 && col+l > width {
			starts = append(starts, i)
			col = 0
		}
		col += l + 1
	}
	return starts
}

func (m *Model) viewTyping() string {
	s := m.sess
	tw := m.textWidth()
	now := time.Now()

	// Words longer than the line are shown up to the width.
	starts := wrap(s.Words, tw)
	lineOf := func(word int) int {
		l := 0
		for i, st := range starts {
			if st <= word {
				l = i
			}
		}
		return l
	}
	cur := lineOf(s.Cur)
	first := max(cur-1, 0)

	ghost := -1
	if m.activity == ActPace && m.phase == phaseBurst && s.Started() {
		ghost = int(m.target * 5 / 60 * s.Elapsed(now).Seconds())
	}

	var lines []string
	offset := 0 // absolute character offset of each word, for the ghost
	wordOffset := make([]int, len(s.Words))
	for i := range s.Words {
		wordOffset[i] = offset
		offset += len(s.Words[i]) + 1
	}
	for li := first; li < first+3 && li < len(starts); li++ {
		end := len(s.Words)
		if li+1 < len(starts) {
			end = starts[li+1]
		}
		var cells []cell
		for wi := starts[li]; wi < end; wi++ {
			cells = append(cells, m.wordCells(wi, wordOffset[wi], ghost)...)
		}
		lines = append(lines, m.renderCells(cells, tw))
	}
	for len(lines) < 3 {
		lines = append(lines, m.bgSpaces(tw))
	}

	out := []string{m.pad(m.typingHeader(now), tw), m.bgSpaces(tw)}
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
		c := cell{r: r, fg: m.th.Sub, bg: m.th.BgHex(), under: wrong}
		if j < len(in) {
			if in[j] == r {
				c.fg = m.th.Text
			} else {
				c.fg = m.th.Error
			}
		}
		if off+j == ghost {
			c.bg = m.th.SubAlt
			c.fg = m.warm(m.th.Main)
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
	sp := cell{r: ' ', fg: m.th.Sub, bg: m.th.BgHex()}
	if off+len(word) == ghost {
		sp.bg = m.th.SubAlt
	}
	return append(cells, sp)
}

// renderCells groups runs of identical style so lipgloss isn't called per character.
func (m *Model) renderCells(cells []cell, width int) string {
	if len(cells) > width {
		cells = cells[:width]
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
		left = m.main(fmt.Sprintf("%d", int((s.TimeLimit-s.Elapsed(now)).Seconds()+0.999)))
	default:
		left = m.main(fmt.Sprintf("%d/%d", s.Cur, len(s.Words)))
	}
	if s.Started() && m.cfg.LiveWPM {
		live := "–"
		if w := s.LiveWPM(now); w > 0 {
			live = fmt.Sprintf("%.0f", w)
		}
		left += m.bgSpaces(3) + m.fg(m.warm(m.th.Text)).Render(live) + m.sub(" wpm")
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
		left += m.fg(m.hot(m.th.Sub)).Render("   train · ") + m.fg(m.warm(m.th.Main)).Render(strings.Join(units, "  "))
	case ActPace:
		if m.phase == phaseBurst {
			left += m.fg(m.hot(m.th.Sub)).Render("   burst · ") + m.sub("pacer ") + m.fg(m.warm(m.th.Main)).Render(fmt.Sprintf("%.0f", m.target)) + m.sub(" wpm")
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
			left += m.fg(m.hot(m.th.Sub)).Render("   control · ") + m.sub("type clean, no pacer")
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
		punct = m.fg(m.warm(m.th.Main)).Render("@ punctuation")
	}
	if m.cfg.Numbers {
		nums = m.fg(m.hot(m.th.Main)).Render("# numbers")
	}
	return punct + sep + nums + sep + m.main(mode)
}

func (m *Model) typingFooter() string {
	return m.keycap("tab", m.warm(m.th.Text)) + m.sub(" restart   ") + m.keycap("esc", m.hot(m.th.Text)) + m.sub(" menu")
}
