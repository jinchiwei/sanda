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

type menuItem struct {
	section string
	label   string
	active  func(m *Model) bool
	do      func(m *Model) tea.Cmd
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

func (m *Model) buildMenu() {
	var items []menuItem
	for _, t := range []int{15, 30, 60, 120} {
		t := t
		items = append(items, menuItem{"time", fmt.Sprint(t),
			func(m *Model) bool { return m.cfg.Mode == "time" && m.cfg.Time == t },
			m.setTest("time", func(c *config.Config) { c.Time = t })})
	}
	for _, w := range []int{10, 25, 50, 100} {
		w := w
		items = append(items, menuItem{"words", fmt.Sprint(w),
			func(m *Model) bool { return m.cfg.Mode == "words" && m.cfg.Words == w },
			m.setTest("words", func(c *config.Config) { c.Words = w })})
	}
	for _, q := range []string{"all", "short", "medium", "long", "thicc"} {
		q := q
		items = append(items, menuItem{"quote", q,
			func(m *Model) bool { return m.cfg.Mode == "quote" && m.cfg.QuoteLength == q },
			m.setTest("quote", func(c *config.Config) { c.QuoteLength = q })})
	}
	items = append(items,
		menuItem{"options", "punctuation", func(m *Model) bool { return m.cfg.Punctuation },
			func(m *Model) tea.Cmd { m.cfg.Punctuation = !m.cfg.Punctuation; m.saveConfig(); return nil }},
		menuItem{"options", "numbers", func(m *Model) bool { return m.cfg.Numbers },
			func(m *Model) tea.Cmd { m.cfg.Numbers = !m.cfg.Numbers; m.saveConfig(); return nil }},
		menuItem{"options", "live wpm", func(m *Model) bool { return m.cfg.LiveWPM },
			func(m *Model) tea.Cmd { m.cfg.LiveWPM = !m.cfg.LiveWPM; m.saveConfig(); return nil }},
		menuItem{"practice", "train · adaptive drills on your weak spots", nil,
			func(m *Model) tea.Cmd { m.startTrain(); return nil }},
		menuItem{"practice", "pace · overspeed bursts + clean control rounds", nil,
			func(m *Model) tea.Cmd { m.activity = ActTest; m.startPace(); return nil }},
		menuItem{"app", "progress", nil, func(m *Model) tea.Cmd { m.openProgress(scrMenu); return nil }},
		menuItem{"app", "theme · " + m.cfg.Theme, nil, func(m *Model) tea.Cmd { m.openThemes(scrMenu); return nil }},
		menuItem{"app", "quit", nil, func(m *Model) tea.Cmd { return tea.Quit }},
	)
	m.menuItems = items
}

func (m *Model) openMenu(from screen) {
	m.back = from
	m.buildMenu()
	m.screen = scrMenu
}

func (m *Model) keyMenu(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "up", "k", "left", "h", "shift+tab":
		m.menuIdx = (m.menuIdx - 1 + len(m.menuItems)) % len(m.menuItems)
	case "down", "j", "right", "l", "tab":
		m.menuIdx = (m.menuIdx + 1) % len(m.menuItems)
	case "enter", " ":
		cmd := m.menuItems[m.menuIdx].do(m)
		if m.screen == scrMenu {
			m.buildMenu()
		}
		return m, cmd
	case "esc":
		m.screen = m.back
		if m.screen == scrTyping && m.sess.Finished() {
			m.restart()
		}
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) viewMenu() string {
	out := []string{m.main("sanda 散打"), m.bgSpaces(1)}
	section := ""
	var row string
	flush := func() {
		if row != "" {
			out = append(out, row)
			row = ""
		}
	}
	for i, it := range m.menuItems {
		if it.section != section {
			flush()
			section = it.section
			row = m.sub(fmt.Sprintf("%-9s", section))
		} else if it.section == "practice" || it.section == "app" {
			flush()
			row = m.bgSpaces(9)
		}
		label := it.label
		var st lipgloss.Style
		switch {
		case it.active != nil && it.active(m):
			st = m.fg(m.th.Main)
		default:
			st = m.fg(m.th.Text)
		}
		if i == m.menuIdx {
			st = st.Background(lipgloss.Color(m.th.SubAlt)).Foreground(lipgloss.Color(m.th.Caret))
		}
		row += st.Render(" "+label+" ") + m.bgSpaces(1)
	}
	flush()
	out = append(out, m.bgSpaces(1), m.sub("↑↓ move · enter select · esc back"))
	if m.status != "" {
		out = append(out, m.fg(m.th.Error).Render(m.status))
	}
	return m.block(out)
}

// ---- themes ----

func (m *Model) openThemes(from screen) {
	m.back = from
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
		m.th = theme.Get(m.themeBefore)
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
		m.th = theme.Get(list[m.themeIdx]) // live preview
	}
	return m, nil
}

func (m *Model) closeThemes() {
	m.screen = m.back
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
		for _, c := range []string{t.Bg, t.Main, t.Caret, t.Text, t.Sub, t.Error} {
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
		m.text("the quick ") + m.fg(m.th.Error).Render("x") + m.text("rown ") + m.fg(m.th.Bg).Background(lipgloss.Color(m.th.Caret)).Render("f") + m.sub("ox jumps over"),
	}
	out = append(out, sample...)
	return m.block(out)
}

// ---- progress ----

func (m *Model) openProgress(from screen) {
	m.back = from
	m.progressW = 0 // rebuilt on the next View, once the terminal size is known
	m.screen = scrProgress
}

func (m *Model) keyProgress(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", "enter":
		m.screen = m.back
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
	out = append(out, m.sub("the gap · median wpm per week (word/time tests vs quotes)"))
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
	out = append(out, m.sub(fmt.Sprintf("transition types · your median key-to-key time is %dms (last 90 days, %d samples)", md.Baseline.Milliseconds(), md.Samples)))
	for _, c := range md.Classes {
		diff := float64(c.Median-md.Baseline) / float64(md.Baseline) * 100
		col := m.th.Text
		if diff > 15 {
			col = m.th.Error
		}
		out = append(out, m.fg(col).Render(fmt.Sprintf("  %-34s %5dms  %+4.0f%%  (%d)", c.Class, c.Median.Milliseconds(), diff, c.Count)))
	}

	sched, _ := m.db.Schedule()
	out = append(out, m.bgSpaces(1), m.sub("top weak spots · estimated time lost per 1000 characters of typical text"))
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
		out = append(out, m.text(row)+m.fg(m.th.Main).Render(fmt.Sprintf("%-14s", spark(daily)))+m.sub(" "+rev))
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
