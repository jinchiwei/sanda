// sanda (散打): a terminal typing trainer that finds what slows you down and drills it.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jinchiwei/sanda/internal/config"
	"github.com/jinchiwei/sanda/internal/data"
	"github.com/jinchiwei/sanda/internal/store"
	"github.com/jinchiwei/sanda/internal/theme"
	"github.com/jinchiwei/sanda/internal/ui"
)

var version = "dev"

const usage = `sanda (散打): a terminal typing trainer

usage:
  sanda [flags]          take a test (Monkeytype-style)
  sanda train            adaptive drills on your weakest transitions
  sanda pace             overspeed bursts alternating with clean control rounds
  sanda progress         your plateau gap and weak spots over time
  sanda themes           pick a theme (live preview)
  sanda history [n]      print your last n official tests (default 20)

flags (saved as your new defaults):
  -t N        time test, N seconds (15, 30, 60, 120)
  -w N        word test, N words (10, 25, 50, 100)
  -q LENGTH   quote test: all, short, medium, long, thicc
  -p          toggle punctuation
  -n          toggle numbers
  -theme NAME
  -version

keys: tab restart · esc menu · ctrl+c quit
files: ~/.config/sanda/config.toml, ~/.config/sanda/themes/*.toml, ~/.local/share/sanda/sanda.db
`

func main() {
	fs := flag.NewFlagSet("sanda", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	t := fs.Int("t", 0, "")
	w := fs.Int("w", 0, "")
	q := fs.String("q", "", "")
	p := fs.Bool("p", false, "")
	n := fs.Bool("n", false, "")
	th := fs.String("theme", "", "")
	ver := fs.Bool("version", false, "")

	args := os.Args[1:]
	cmd := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	fs.Parse(args)
	if *ver {
		fmt.Println("sanda", version)
		return
	}
	switch cmd {
	case "", "test", "train", "pace", "progress", "themes", "history":
	case "help":
		fmt.Print(usage)
		return
	default:
		fail(fmt.Errorf("unknown command %q\n\n%s", cmd, usage))
	}
	if err := data.Err(); err != nil {
		fail(err)
	}

	cfg := config.Load()
	changed := false
	switch {
	case *t > 0:
		cfg.Mode, cfg.Time, changed = "time", *t, true
	case *w > 0:
		cfg.Mode, cfg.Words, changed = "words", *w, true
	case *q != "":
		switch data.QuoteLength(*q) {
		case data.QuoteAll, data.QuoteShort, data.QuoteMedium, data.QuoteLong, data.QuoteThicc:
		default:
			fail(fmt.Errorf("unknown quote length %q (all, short, medium, long, thicc)", *q))
		}
		cfg.Mode, cfg.QuoteLength, changed = "quote", *q, true
	}
	if *p {
		cfg.Punctuation, changed = !cfg.Punctuation, true
	}
	if *n {
		cfg.Numbers, changed = !cfg.Numbers, true
	}
	if *th != "" {
		if _, ok := theme.Load()[*th]; !ok {
			fail(fmt.Errorf("unknown theme %q (see: sanda themes)", *th))
		}
		cfg.Theme, changed = *th, true
	}
	if changed {
		if err := config.Save(cfg); err != nil {
			fail(err)
		}
	}

	db, err := store.Open(store.DefaultPath())
	if err != nil {
		fail(err)
	}
	defer db.Close()
	if cmd == "history" {
		history(db, fs.Args())
		return
	}
	ui.Version = version
	if _, err := tea.NewProgram(ui.New(cfg, db, cmd), tea.WithAltScreen()).Run(); err != nil {
		fail(err)
	}
}

func history(db *store.DB, args []string) {
	limit := 20
	if len(args) > 0 {
		fmt.Sscan(args[0], &limit)
	}
	tests, err := db.Tests(time.Time{}, store.KindOfficial)
	if err != nil {
		fail(err)
	}
	if len(tests) > limit {
		tests = tests[len(tests)-limit:]
	}
	fmt.Printf("%-17s %-14s %6s %6s %6s %6s\n", "when", "test", "wpm", "raw", "acc", "cons")
	for _, t := range tests {
		label := t.Mode + " " + t.ModeValue
		if t.Punctuation {
			label += " @"
		}
		if t.Numbers {
			label += " #"
		}
		fmt.Printf("%-17s %-14s %6.1f %6.1f %5.1f%% %5.0f%%\n", t.StartedAt.Local().Format("2006-01-02 15:04"), label, t.WPM, t.Raw, t.Acc, t.Cons)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "sanda:", err)
	os.Exit(1)
}
