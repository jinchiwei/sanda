package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jinchiwei/sanda/internal/engine"
)

// bgStyle sets a background unless hex is "" (terminal background).
func bgStyle(st lipgloss.Style, hex string) lipgloss.Style {
	if hex == "" {
		return st
	}
	return st.Background(lipgloss.Color(hex))
}

// fg returns a style with the given foreground on the theme background (every cell must carry the background).
func (m *Model) fg(hex string) lipgloss.Style {
	return bgStyle(lipgloss.NewStyle().Foreground(lipgloss.Color(hex)), m.th.BgHex())
}

func (m *Model) sub(s string) string  { return m.fg(m.th.Sub).Render(s) }
func (m *Model) main(s string) string { return m.fg(m.th.Main).Render(s) }
func (m *Model) text(s string) string { return m.fg(m.th.Text).Render(s) }
func (m *Model) bgSpaces(n int) string {
	if n <= 0 {
		return ""
	}
	return m.fg(m.th.Text).Render(strings.Repeat(" ", n))
}

// pad right-pads a rendered string to width w with background-coloured spaces.
func (m *Model) pad(s string, w int) string {
	return s + m.bgSpaces(w-lipgloss.Width(s))
}

// block pads every line of a multi-line string to the same width so the background is continuous.
func (m *Model) block(lines []string) string {
	w := 0
	for _, l := range lines {
		w = max(w, lipgloss.Width(l))
	}
	for i := range lines {
		lines[i] = m.pad(lines[i], w)
	}
	return strings.Join(lines, "\n")
}

// 3×5 pixel digits, drawn two pixel rows per text row with half blocks.
var font = map[rune][5]string{
	'0': {"111", "101", "101", "101", "111"},
	'1': {"010", "110", "010", "010", "111"},
	'2': {"111", "001", "111", "100", "111"},
	'3': {"111", "001", "111", "001", "111"},
	'4': {"101", "101", "111", "001", "001"},
	'5': {"111", "100", "111", "001", "111"},
	'6': {"111", "100", "111", "101", "111"},
	'7': {"111", "001", "001", "001", "001"},
	'8': {"111", "101", "111", "101", "111"},
	'9': {"111", "101", "111", "001", "111"},
	'%': {"101", "001", "010", "100", "101"},
	'.': {"0", "0", "0", "0", "1"},
	' ': {"0", "0", "0", "0", "0"},
}

// big renders a short number in the half-block font (3 rows tall).
func (m *Model) big(s string, hex string) []string {
	rows := [6]string{}
	for i, r := range s {
		g, ok := font[r]
		if !ok {
			continue
		}
		for y := 0; y < 5; y++ {
			if i > 0 {
				rows[y] += "0"
			}
			rows[y] += g[y]
		}
	}
	rows[5] = strings.Repeat("0", len(rows[0]))
	out := make([]string, 3)
	for ty := 0; ty < 3; ty++ {
		top, bot := rows[ty*2], rows[ty*2+1]
		var sb strings.Builder
		for x := 0; x < len(top); x++ {
			t, b := top[x] == '1', x < len(bot) && bot[x] == '1'
			switch {
			case t && b:
				sb.WriteRune('█')
			case t:
				sb.WriteRune('▀')
			case b:
				sb.WriteRune('▄')
			default:
				sb.WriteRune(' ')
			}
		}
		out[ty] = m.fg(hex).Render(sb.String())
	}
	return out
}

// chart draws WPM (main) and raw (sub) per second, with error marks (error colour) on the axis line.
func (m *Model) chart(ps []engine.Second, width, height int) []string {
	if len(ps) == 0 || width < 10 {
		return nil
	}
	top := 0.0
	for _, p := range ps {
		top = math.Max(top, math.Max(p.WPM, p.Raw))
	}
	top = math.Ceil(top/20) * 20
	if top == 0 {
		top = 20
	}
	label := len(fmt.Sprint(int(top))) + 1
	cols := width - label
	grid := make([][]rune, height)
	kind := make([][]int, height) // 0 empty, 1 raw, 2 wpm
	for y := range grid {
		grid[y] = []rune(strings.Repeat(" ", cols))
		kind[y] = make([]int, cols)
	}
	val := func(x int, f func(engine.Second) float64) float64 {
		pos := float64(x) / float64(max(cols-1, 1)) * float64(len(ps)-1)
		i := int(pos)
		if i >= len(ps)-1 {
			return f(ps[len(ps)-1])
		}
		frac := pos - float64(i)
		return f(ps[i])*(1-frac) + f(ps[i+1])*frac
	}
	row := func(v float64) int {
		y := height - 1 - int(math.Round(v/top*float64(height-1)))
		return min(max(y, 0), height-1)
	}
	for x := 0; x < cols; x++ {
		ry := row(val(x, func(s engine.Second) float64 { return s.Raw }))
		grid[ry][x], kind[ry][x] = '·', 1
		wy := row(val(x, func(s engine.Second) float64 { return s.WPM }))
		grid[wy][x], kind[wy][x] = '•', 2
	}
	var out []string
	for y := 0; y < height; y++ {
		var lab string
		switch y {
		case 0:
			lab = fmt.Sprint(int(top))
		case height - 1:
			lab = "0"
		}
		line := m.sub(fmt.Sprintf("%*s ", label-1, lab))
		for x := 0; x < cols; x++ {
			c := string(grid[y][x])
			switch kind[y][x] {
			case 2:
				line += m.main(c)
			case 1:
				line += m.sub(c)
			default:
				line += m.bgSpaces(1)
			}
		}
		out = append(out, line)
	}
	// Axis with error marks.
	axis := []rune(strings.Repeat("─", cols))
	errAt := map[int]bool{}
	for i, p := range ps {
		if p.Errors > 0 {
			x := int(math.Round(float64(i) / float64(max(len(ps)-1, 1)) * float64(cols-1)))
			errAt[x] = true
		}
	}
	line := m.bgSpaces(label)
	for x, r := range axis {
		if errAt[x] {
			line += m.fg(m.th.Error).Render("x")
		} else {
			line += m.fg(m.th.SubAlt).Render(string(r))
		}
	}
	out = append(out, line)
	return out
}

// spark draws values as a sparkline (taller = larger).
func spark(v []float64) string {
	if len(v) == 0 {
		return ""
	}
	lo, hi := v[0], v[0]
	for _, x := range v {
		lo, hi = math.Min(lo, x), math.Max(hi, x)
	}
	bars := []rune("▁▂▃▄▅▆▇█")
	var sb strings.Builder
	for _, x := range v {
		i := 0
		if hi > lo {
			i = int((x - lo) / (hi - lo) * float64(len(bars)-1))
		}
		sb.WriteRune(bars[i])
	}
	return sb.String()
}
