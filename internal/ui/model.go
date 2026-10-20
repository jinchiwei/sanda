// Package ui is sanda's Bubble Tea interface.
package ui

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jinchiwei/sanda/internal/analysis"
	"github.com/jinchiwei/sanda/internal/config"
	"github.com/jinchiwei/sanda/internal/data"
	"github.com/jinchiwei/sanda/internal/engine"
	"github.com/jinchiwei/sanda/internal/store"
	"github.com/jinchiwei/sanda/internal/textgen"
	"github.com/jinchiwei/sanda/internal/theme"
	"github.com/jinchiwei/sanda/internal/train"
)

type screen int

const (
	scrTyping screen = iota
	scrResult
	scrMenu
	scrThemes
	scrProgress
)

// Activity is what the typing screen is for.
type Activity int

const (
	ActTest Activity = iota
	ActTrain
	ActPace
)

type pacePhase int

const (
	phaseBurst pacePhase = iota
	phaseControl
)

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type Model struct {
	cfg  config.Config
	th   theme.Theme
	db   *store.DB
	w, h int

	screen   screen
	back     screen // where Esc returns from menu/themes/progress
	activity Activity

	sess   *engine.Session
	words  []string
	quote  *data.Quote
	gen    *textgen.Gen
	saved  bool
	seed   uint64
	status string

	// results
	stats   engine.Stats
	losses  []analysis.Loss
	pb      bool
	shownAt time.Time // when results appeared; keys are ignored briefly so a trailing keystroke can't skip them
	prevPB  float64
	outcome []train.Outcome

	// train / pace
	model    analysis.Model
	focus    []train.Focus
	phase    pacePhase
	target   float64
	paceNote string

	// menu
	menuRows []menuRow
	menuRow  int
	menuCol  int

	// themes
	themeNames  []string
	themeIdx    int
	themeFilter string
	themeBefore string

	progress  string
	progressW int // terminal width the progress screen was built for
}

// New creates the app starting in the given activity ("progress" and "themes" open those screens).
func New(cfg config.Config, db *store.DB, start string) *Model {
	m := &Model{cfg: cfg, db: db, seed: uint64(time.Now().UnixNano())}
	m.setTheme(cfg.Theme)
	switch start {
	case "train":
		m.startTrain()
	case "pace":
		m.startPace()
	case "progress":
		m.newTest()
		m.openProgress(scrTyping)
	case "themes":
		m.newTest()
		m.openThemes(scrTyping)
	default:
		m.newTest()
	}
	return m
}

func (m *Model) Init() tea.Cmd { return tick() }

// setTheme loads a theme, honouring the terminal-background option.
func (m *Model) setTheme(name string) {
	m.th = theme.Get(name)
	if m.cfg.TermBg {
		m.th.Bg = theme.Terminal
	}
}

func (m *Model) nextSeed() uint64 { m.seed++; return m.seed }

// newTest starts an official Monkeytype-style test from the config.
func (m *Model) newTest() {
	m.activity = ActTest
	m.quote = nil
	m.gen = textgen.New(m.cfg.Punctuation, m.cfg.Numbers, m.nextSeed())
	switch m.cfg.Mode {
	case "words":
		m.words = m.gen.Words(m.cfg.Words)
		m.sess = engine.New(m.words)
		m.sess.FreeBackspace = m.cfg.FreeBack
	case "quote":
		qs := data.Quotes(data.QuoteLength(m.cfg.QuoteLength))
		if len(qs) == 0 {
			qs = data.Quotes(data.QuoteAll)
		}
		q := qs[rand.New(rand.NewPCG(m.nextSeed(), 7)).IntN(len(qs))]
		m.quote = &q
		m.words = textgen.SplitQuote(q.Text)
		m.sess = engine.New(m.words)
		m.sess.FreeBackspace = m.cfg.FreeBack
	default:
		m.words = m.gen.Words(100)
		m.sess = engine.New(m.words)
		m.sess.FreeBackspace = m.cfg.FreeBack
		m.sess.TimeLimit = time.Duration(m.cfg.Time) * time.Second
		m.sess.More = m.gen.Words
	}
	m.saved = false
	m.screen = scrTyping
}

func (m *Model) reloadModel() {
	rounds, err := m.db.Events(time.Now().AddDate(0, 0, -90))
	if err != nil {
		m.status = "could not read history: " + err.Error()
		return
	}
	m.model = analysis.BuildModel(rounds)
}

func (m *Model) startTrain() {
	m.activity = ActTrain
	m.reloadModel()
	sched, _ := m.db.Schedule()
	m.focus = train.PickFocus(m.model, sched, time.Now(), 3)
	if len(m.focus) == 0 {
		for _, u := range analysis.TopCorpusUnits(3) {
			m.focus = append(m.focus, train.Focus{Unit: u, Probe: true})
		}
	}
	m.words = train.Drill(m.focus, m.cfg.DrillWords, m.nextSeed())
	m.sess = engine.New(m.words)
	m.sess.FreeBackspace = m.cfg.FreeBack
	m.quote = nil
	m.saved = false
	m.screen = scrTyping
}

func (m *Model) startPace() {
	if m.activity != ActPace {
		m.phase = phaseBurst
	}
	m.activity = ActPace
	tests, _ := m.db.Tests(time.Now().AddDate(0, 0, -30), store.KindOfficial)
	m.target = train.PaceTarget(tests, m.cfg.PacerFactor)
	m.gen = textgen.New(m.cfg.Punctuation, m.cfg.Numbers, m.nextSeed())
	m.words = m.gen.Words(100)
	m.sess = engine.New(m.words)
	m.sess.FreeBackspace = m.cfg.FreeBack
	m.sess.More = m.gen.Words
	secs := m.cfg.BurstSecs
	if m.phase == phaseControl {
		secs = m.cfg.ControlSecs
	}
	m.sess.TimeLimit = time.Duration(secs) * time.Second
	m.quote = nil
	m.saved = false
	m.screen = scrTyping
}

// restart begins a fresh round of the current activity.
func (m *Model) restart() {
	switch m.activity {
	case ActTrain:
		m.startTrain()
	case ActPace:
		m.startPace()
	default:
		m.newTest()
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tickMsg:
		if m.screen == scrTyping && m.sess != nil {
			m.sess.Tick(time.Time(msg))
			m.checkDone()
		}
		return m, tick()
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.screen {
		case scrTyping:
			return m.keyTyping(msg)
		case scrResult:
			return m.keyResult(msg)
		case scrMenu:
			return m.keyMenu(msg)
		case scrThemes:
			return m.keyThemes(msg)
		case scrProgress:
			return m.keyProgress(msg)
		}
	}
	return m, nil
}

func (m *Model) keyTyping(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	now := time.Now()
	if k.Paste {
		return m, nil
	}
	switch k.String() {
	case "tab":
		m.restart()
		return m, nil
	case "esc":
		m.openMenu(scrTyping)
		return m, nil
	case "backspace":
		m.sess.Backspace(now)
	case "ctrl+h", "ctrl+w", "alt+backspace", "ctrl+backspace":
		m.sess.DeleteWord(now)
	case " ":
		m.sess.Space(now)
	default:
		if k.Type == tea.KeyRunes && !k.Alt {
			for _, r := range k.Runes {
				m.sess.Type(r, now)
			}
		}
	}
	m.checkDone()
	return m, nil
}

// checkDone saves a finished round and shows its results.
func (m *Model) checkDone() {
	if !m.sess.Finished() || m.saved {
		return
	}
	m.saved = true
	m.stats = m.sess.Stats()
	if len(m.sess.Events) < 2 || m.stats.Duration < time.Second {
		m.restart()
		return
	}
	t := store.Test{
		StartedAt: time.Now().Add(-m.stats.Duration), Text: strings.Join(m.words[:min(len(m.words), len(m.sess.Input))], " "),
		Punctuation: m.cfg.Punctuation, Numbers: m.cfg.Numbers,
		WPM: m.stats.WPM, Raw: m.stats.Raw, Acc: m.stats.Acc, Cons: m.stats.Consistency, DurationMs: m.stats.Duration.Milliseconds(),
		Correct: m.stats.Correct, Incorrect: m.stats.Incorrect, Extra: m.stats.Extra, Missed: m.stats.Missed,
	}
	switch m.activity {
	case ActTest:
		t.Kind = store.KindOfficial
		t.Mode = m.cfg.Mode
		switch m.cfg.Mode {
		case "time":
			t.ModeValue = strconv.Itoa(m.cfg.Time)
		case "words":
			t.ModeValue = strconv.Itoa(m.cfg.Words)
		case "quote":
			t.ModeValue = m.cfg.QuoteLength
			if m.quote != nil {
				t.QuoteID = m.quote.ID
			}
		}
		m.prevPB = m.db.BestWPM(t.Mode, t.ModeValue, t.Punctuation, t.Numbers)
		m.pb = m.stats.WPM > m.prevPB
	case ActTrain:
		t.Kind, t.Mode, t.ModeValue = store.KindTrain, "drill", strconv.Itoa(len(m.words))
		var units []string
		for _, f := range m.focus {
			units = append(units, f.Unit)
		}
		t.Focus = strings.Join(units, "\x1f")
	case ActPace:
		t.Kind, t.Mode = store.KindPace, "burst"
		if m.phase == phaseControl {
			t.Mode = "control"
		}
		t.ModeValue = fmt.Sprintf("%.0f", m.target)
	}
	if _, err := m.db.SaveTest(t, m.sess.Events); err != nil {
		m.status = "could not save: " + err.Error()
	}
	m.losses = analysis.Breakdown(m.sess.Events, m.words, m.stats.Duration)
	m.outcome = nil
	if m.activity == ActTrain {
		m.reloadModel()
		m.outcome = train.Review(m.db, m.focus, m.sess.Events, m.model.Baseline, time.Now())
	}
	if m.activity == ActPace {
		m.paceNote = m.paceVerdict()
	}
	m.shownAt = time.Now()
	m.screen = scrResult
}

func (m *Model) paceVerdict() string {
	if m.phase == phaseBurst {
		if m.stats.WPM >= m.target {
			return fmt.Sprintf("kept up with the pacer (%.0f ≥ %.0f)", m.stats.WPM, m.target)
		}
		return fmt.Sprintf("%.0f WPM behind the pacer", m.target-m.stats.WPM)
	}
	if m.stats.Acc >= 97 {
		return fmt.Sprintf("clean control round (%.0f%% accuracy)", m.stats.Acc)
	}
	return fmt.Sprintf("control round below 97%% accuracy (%.0f%%): slow down a touch", m.stats.Acc)
}

func (m *Model) keyResult(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if time.Since(m.shownAt) < 500*time.Millisecond {
		return m, nil
	}
	switch k.String() {
	case "tab", "enter":
		if m.activity == ActPace {
			if m.phase == phaseBurst {
				m.phase = phaseControl
			} else {
				m.phase = phaseBurst
			}
		}
		m.restart()
	case "esc":
		m.openMenu(scrResult)
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) View() string {
	if m.w == 0 {
		return ""
	}
	var body string
	switch m.screen {
	case scrTyping:
		body = m.viewTyping()
	case scrResult:
		body = m.viewResult()
	case scrMenu:
		body = m.viewMenu()
	case scrThemes:
		body = m.viewThemes()
	case scrProgress:
		if m.progressW != m.w {
			m.progress, m.progressW = m.buildProgress(), m.w
		}
		body = m.progress
	}
	if m.th.BgHex() == "" {
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, body)
	}
	return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, body,
		lipgloss.WithWhitespaceBackground(lipgloss.Color(m.th.Bg)))
}

// textWidth is the width of the typing area.
func (m *Model) textWidth() int { return min(max(m.w-8, 20), 88) }
