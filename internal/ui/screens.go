package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jinchiwei/sanda/internal/analysis"
	"github.com/jinchiwei/sanda/internal/config"
	"github.com/jinchiwei/sanda/internal/store"
	"github.com/jinchiwei/sanda/internal/theme"
)

// ---- menu ----

// The menu is a grid: each row is a group of choices. j/k (or ↓/↑) move between rows, h/l (or ←/→) within a row.
type menuItem struct {
	label  string
	active func(m *Model) bool
	do     func(m *Model) tea.Cmd
}

type menuRow struct {
	section string // shown only on the first row of a section
	items   []menuItem
}

func (m *Model) setTest(mode string, apply func(c *config.Config)) func(*Model) tea.Cmd {
	return func(m *Model) tea.Cmd {
		m.cfg.Mode = mode
		apply(&m.cfg)
		m.saveConfig()
		m.newTest()
		return nil
	}
}

func (m *Model) saveConfig() {
	if err := config.Save(m.cfg); err != nil {
		m.status = "could not save settings: " + err.Error()
	}
}

func toggle(label string, field func(c *config.Config) *bool) menuItem {
	return menuItem{label,
		func(m *Model) bool { return *field(&m.cfg) },
		func(m *Model) tea.Cmd { f := field(&m.cfg); *f = !*f; m.saveConfig(); return nil }}
}

func (m *Model) buildMenu() {
	var timeRow, wordsRow, quoteRow []menuItem
	for _, t := range []int{15, 30, 60, 120} {
		t := t
		timeRow = append(timeRow, menuItem{fmt.Sprint(t),
			func(m *Model) bool { return m.cfg.Mode == "time" && m.cfg.Time == t },
			m.setTest("time", func(c *config.Config) { c.Time = t })})
	}
	for _, w := range []int{10, 25, 50, 100} {
		w := w
		wordsRow = append(wordsRow, menuItem{fmt.Sprint(w),
			func(m *Model) bool { return m.cfg.Mode == "words" && m.cfg.Words == w },
			m.setTest("words", func(c *config.Config) { c.Words = w })})
	}
	for _, q := range []string{"all", "short", "medium", "long", "thicc"} {
		q := q
		quoteRow = append(quoteRow, menuItem{q,
			func(m *Model) bool { return m.cfg.Mode == "quote" && m.cfg.QuoteLength == q },
			m.setTest("quote", func(c *config.Config) { c.QuoteLength = q })})
	}
	m.menuRows = []menuRow{
		{"time", timeRow},
		{"words", wordsRow},
		{"quote", quoteRow},
		{"options", []menuItem{
			toggle("punctuation", func(c *config.Config) *bool { return &c.Punctuation }),
			toggle("numbers", func(c *config.Config) *bool { return &c.Numbers }),
			toggle("live wpm", func(c *config.Config) *bool { return &c.LiveWPM }),
			toggle("free backspace", func(c *config.Config) *bool { return &c.FreeBack }),
			{"terminal bg", func(m *Model) bool { return m.cfg.TermBg },
				func(m *Model) tea.Cmd {
					m.cfg.TermBg = !m.cfg.TermBg
					m.saveConfig()
					m.setTheme(m.cfg.Theme)
					return nil
				}},
		}},
		{"practice", []menuItem{{"train · adaptive drills on your weak spots", nil,
			func(m *Model) tea.Cmd { m.startTrain(); return nil }}}},
		{"", []menuItem{{"pace · overspeed bursts + clean control rounds", nil,
			func(m *Model) tea.Cmd { m.activity = ActTest; m.startPace(); return nil }}}},
		{"app", []menuItem{{"progress", nil, func(m *Model) tea.Cmd { m.openProgress(scrMenu); return nil }}}},
		{"", []menuItem{{"theme · " + m.cfg.Theme, nil, func(m *Model) tea.Cmd { m.openThemes(scrMenu); return nil }}}},
		{"", []menuItem{{"quit", nil, func(m *Model) tea.Cmd { return tea.Quit }}}},
	}
	m.menuRow = min(m.menuRow, len(m.menuRows)-1)
	m.menuCol = min(m.menuCol, len(m.menuRows[m.menuRow].items)-1)
}

func (m *Model) openMenu(from screen) {
	m.menuBack = from
	m.buildMenu()
	m.screen = scrMenu
}

func (m *Model) keyMenu(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if k.Type == tea.KeyRunes && len(k.Runes) > 1 { // fast key repeats can arrive as one event
		var cmd tea.Cmd
		for _, r := range k.Runes {
			_, c := m.keyMenu(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			if c != nil {
				cmd = c
			}
			if m.screen != scrMenu {
				break
			}
		}
		return m, cmd
	}
	rows := len(m.menuRows)
	clampCol := func() { m.menuCol = min(m.menuCol, len(m.menuRows[m.menuRow].items)-1) }
	switch k.String() {
	case "up", "k":
		m.menuRow = (m.menuRow - 1 + rows) % rows
		clampCol()
	case "down", "j":
		m.menuRow = (m.menuRow + 1) % rows
		clampCol()
	case "left", "h":
		m.menuCol = max(m.menuCol-1, 0)
	case "right", "l":
		m.menuCol = min(m.menuCol+1, len(m.menuRows[m.menuRow].items)-1)
	case "tab": // walk every item in order
		if m.menuCol+1 < len(m.menuRows[m.menuRow].items) {
			m.menuCol++
		} else {
			m.menuRow, m.menuCol = (m.menuRow+1)%rows, 0
		}
	case "shift+tab":
		if m.menuCol > 0 {
			m.menuCol--
		} else {
			m.menuRow = (m.menuRow - 1 + rows) % rows
			m.menuCol = len(m.menuRows[m.menuRow].items) - 1
		}
	case "enter", " ":
		cmd := m.menuRows[m.menuRow].items[m.menuCol].do(m)
		if m.screen == scrMenu {
			m.buildMenu()
		}
		return m, cmd
	case "esc":
		m.screen = m.menuBack
		// A test left mid-way kept its clock running, so start a fresh one.
		if m.screen == scrTyping && m.sess.Started() {
			m.restart()
		}
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) viewMenu() string {
	out := []string{m.main("menu"), m.bgSpaces(1)}
	for r, row := range m.menuRows {
		line := m.fg(m.hot(m.th.Sub)).Render(fmt.Sprintf("%-9s", row.section))
		for c, it := range row.items {
			st := m.fg(m.th.Text)
			if it.active != nil && it.active(m) {
				st = m.fg(m.th.Main)
			}
			if r == m.menuRow && c == m.menuCol {
				st = st.Background(lipgloss.Color(m.th.SubAlt)).Foreground(lipgloss.Color(m.th.Caret))
			}
			line += st.Render(" "+it.label+" ") + m.bgSpaces(1)
		}
		out = append(out, line)
	}
	out = append(out, m.bgSpaces(1), m.sub("j/k ↓↑ rows · h/l ←→ within a row · enter select · esc back"))
	if m.status != "" {
		out = append(out, m.fg(m.th.Error).Render(m.status))
	}
	return m.block(out)
}

// ---- themes ----

func (m *Model) openThemes(from screen) {
	m.subBack = from
	m.themeBefore = m.cfg.Theme
	m.themeFilter = ""
	m.themeNames = theme.Names()
	m.themeIdx = 0
	for i, n := range m.themeNames {
		if n == m.cfg.Theme {
			m.themeIdx = i
		}
	}
	m.screen = scrThemes
}

func (m *Model) filteredThemes() []string {
	if m.themeFilter == "" {
		return m.themeNames
	}
	var out []string
	for _, n := range m.themeNames {
		if strings.Contains(n, strings.ToLower(m.themeFilter)) {
			out = append(out, n)
		}
	}
	return out
}

func (m *Model) keyThemes(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	list := m.filteredThemes()
	switch k.String() {
	case "up", "shift+tab":
		if len(list) > 0 {
			m.themeIdx = (m.themeIdx - 1 + len(list)) % len(list)
		}
	case "down", "tab":
		if len(list) > 0 {
			m.themeIdx = (m.themeIdx + 1) % len(list)
		}
	case "enter":
		m.cfg.Theme = m.th.Name
		m.saveConfig()
		m.closeThemes()
		return m, nil
	case "esc":
		m.setTheme(m.themeBefore)
		m.closeThemes()
		return m, nil
	case "backspace":
		if m.themeFilter != "" {
			m.themeFilter = string([]rune(m.themeFilter)[:len([]rune(m.themeFilter))-1])
			m.themeIdx = 0
		}
	default:
		if k.Type == tea.KeyRunes {
			m.themeFilter += string(k.Runes)
			m.themeIdx = 0
		}
	}
	list = m.filteredThemes()
	if len(list) > 0 {
		m.themeIdx = min(m.themeIdx, len(list)-1)
		m.setTheme(list[m.themeIdx]) // live preview
	}
	return m, nil
}

func (m *Model) closeThemes() {
	m.screen = m.subBack
	if m.screen == scrMenu {
		m.buildMenu()
	}
}

func (m *Model) viewThemes() string {
	list := m.filteredThemes()
	rows := max(m.h-12, 5)
	start := max(0, m.themeIdx-rows/2)
	end := min(len(list), start+rows)
	out := []string{m.main("theme") + m.sub("  type to filter: ") + m.text(m.themeFilter) + m.fg(m.th.Caret).Render("▏"), m.bgSpaces(1)}
	for i := start; i < end; i++ {
		t := theme.Get(list[i])
		swatch := ""
		for _, c := range []string{t.BgHex(), t.Main, t.Caret, t.Text, t.Sub, t.Error} {
			if c == "" {
				swatch += m.sub("··") // terminal background
				continue
			}
			swatch += lipgloss.NewStyle().Background(lipgloss.Color(c)).Render("  ")
		}
		name := fmt.Sprintf(" %-24s", list[i])
		if i == m.themeIdx {
			name = m.fg(m.th.Caret).Background(lipgloss.Color(m.th.SubAlt)).Render(name)
		} else {
			name = m.text(name)
		}
		out = append(out, swatch+name)
	}
	if len(list) == 0 {
		out = append(out, m.sub("no matching themes"))
	}
	out = append(out, m.bgSpaces(1), m.sub(fmt.Sprintf("%d themes · ↑↓ preview · enter keep · esc cancel", len(list))))
	sample := []string{m.bgSpaces(1),
		m.text("the quick ") + m.fg(m.th.Error).Render("x") + m.text("rown ") + m.fg(m.th.OnCaret()).Background(lipgloss.Color(m.th.Caret)).Render("f") + m.sub("ox jumps over"),
	}
	out = append(out, sample...)
	return m.block(out)
}

// ---- progress ----

func (m *Model) openProgress(from screen) {
	m.subBack = from
	m.progressW = 0 // rebuilt on the next View, once the terminal size is known
	m.screen = scrProgress
}

func (m *Model) keyProgress(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", "enter":
		m.screen = m.subBack
		if m.screen == scrMenu {
			m.buildMenu()
		}
	case "tab":
		m.newTest()
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func medianF(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

func (m *Model) buildProgress() string {
	tw := m.textWidth()
	out := []string{m.main("progress"), m.bgSpaces(1)}
	tests, err := m.db.Tests(time.Now().AddDate(0, 0, -56), store.KindOfficial)
	if err != nil {
		return m.block([]string{m.fg(m.th.Error).Render(err.Error())})
	}
	// The plateau metric: quote speed vs short word/time speed, per week.
	out = append(out, m.fg(m.hot(m.th.Sub)).Render("the gap · median wpm per week (word/time tests vs quotes)"))
	out = append(out, m.sub(fmt.Sprintf("%-12s %8s %8s %8s %6s", "week of", "words", "quotes", "gap", "tests")))
	type wk struct {
		words, quotes []float64
	}
	weeks := map[string]*wk{}
	var keys []string
	for _, t := range tests {
		d := t.StartedAt.Local()
		start := d.AddDate(0, 0, -int(d.Weekday())).Format("Jan 02")
		w := weeks[start]
		if w == nil {
			w = &wk{}
			weeks[start] = w
			keys = append(keys, start)
		}
		if t.Mode == "quote" {
			w.quotes = append(w.quotes, t.WPM)
		} else {
			w.words = append(w.words, t.WPM)
		}
	}
	if len(keys) == 0 {
		out = append(out, m.text("no official tests yet: take a few (time, words and quote) to start the record"))
	}
	f := func(v float64) string {
		if v == 0 {
			return "–"
		}
		return fmt.Sprintf("%.0f", v)
	}
	for _, k := range keys {
		w := weeks[k]
		a, b := medianF(w.words), medianF(w.quotes)
		gap := "–"
		if a > 0 && b > 0 {
			gap = fmt.Sprintf("%.0f", a-b)
		}
		out = append(out, m.text(fmt.Sprintf("%-12s %8s %8s %8s %6d", k, f(a), f(b), gap, len(w.words)+len(w.quotes))))
	}

	m.reloadModel()
	md := m.model
	out = append(out, m.bgSpaces(1))
	if md.Samples == 0 {
		out = append(out, m.sub("no keystroke data yet"))
		return m.block(out)
	}
	out = append(out, m.fg(m.hot(m.th.Sub)).Render(fmt.Sprintf("transition types · your median key-to-key time is %dms (last 90 days, %d samples)", md.Baseline.Milliseconds(), md.Samples)))
	for _, c := range md.Classes {
		diff := float64(c.Median-md.Baseline) / float64(md.Baseline) * 100
		col := m.th.Text
		if diff > 15 {
			col = m.th.Error
		}
		out = append(out, m.fg(col).Render(fmt.Sprintf("  %-34s %5dms  %+4.0f%%  (%d)", c.Class, c.Median.Milliseconds(), diff, c.Count)))
	}

	sched, _ := m.db.Schedule()
	out = append(out, m.bgSpaces(1), m.fg(m.hot(m.th.Sub)).Render("top weak spots · estimated time lost per 1000 characters of typical text"))
	out = append(out, m.sub(fmt.Sprintf("  %-6s %8s %6s %6s %7s  %-14s %s", "pair", "median", "err", "seen", "cost", "daily (↓ good)", "review")))
	now := time.Now()
	for i, u := range md.Ranked {
		if i == 10 {
			break
		}
		var daily []float64
		for _, d := range u.Daily {
			daily = append(daily, float64(d.Median))
		}
		if len(daily) > 14 {
			daily = daily[len(daily)-14:]
		}
		rev := "due"
		if s, ok := sched[u.Unit]; ok && s.Due.After(now) {
			rev = "in " + days(s.Due.Sub(now))
		}
		row := fmt.Sprintf("  %-6s %6dms %5.0f%% %6d %6.2fs  ", analysis.Show(u.Unit), u.Median.Milliseconds(), u.ErrRate*100, u.Count, u.Cost)
		out = append(out, m.text(row)+m.fg(m.warm(m.th.Main)).Render(fmt.Sprintf("%-14s", spark(daily)))+m.sub(" "+rev))
	}
	grad := 0
	for _, s := range sched {
		if s.Level >= 2 && s.Due.After(now) {
			grad++
		}
	}
	out = append(out, m.bgSpaces(1), m.sub(fmt.Sprintf("weak spots passed at least twice and not due yet: %d", grad)))
	out = append(out, m.bgSpaces(1), m.sub("esc back · tab new test · q quit"))
	for i := range out {
		if lipgloss.Width(out[i]) > tw+20 {
			out[i] = lipgloss.NewStyle().MaxWidth(tw + 20).Render(out[i])
		}
	}
	return m.block(out)
}
