// Package config loads and saves ~/.config/sanda/config.toml.
package config

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Theme          string  `toml:"theme"`
	Mode           string  `toml:"mode"` // time, words or quote
	Time           int     `toml:"time"`
	Words          int     `toml:"words"`
	QuoteLength    string  `toml:"quote_length"` // short, medium, long, thicc, all
	Punctuation    bool    `toml:"punctuation"`
	Numbers        bool    `toml:"numbers"`
	LiveWPM        bool    `toml:"live_wpm"`            // show the running WPM during a test
	FreeBack       bool    `toml:"free_backspace"`      // backspace into any earlier word, not only wrong ones
	TermBg         bool    `toml:"terminal_background"` // ignore every theme's background and use the terminal's
	KeepLogo       bool    `toml:"keep_logo"`           // keep the 散打 logo on screen while typing
	Window         bool    `toml:"window"`              // always open in a new terminal window (like sanda window)
	WindowFontSize float64 `toml:"window_font_size"`    // font size for that window; 0 = auto
	WindowFont     string  `toml:"window_font"`         // font family for that window; empty = the terminal's
	PacerFactor    float64 `toml:"pacer_factor"`        // pace mode target = recent median × this
	DrillWords     int     `toml:"drill_words"`         // words per train round
	BurstSecs      int     `toml:"burst_seconds"`       // pace mode burst length
	ControlSecs    int     `toml:"control_seconds"`
}

func Default() Config {
	return Config{
		Theme: "dreamtheme", Mode: "time", Time: 30, Words: 50, QuoteLength: "all", LiveWPM: true, FreeBack: true, KeepLogo: true,
		PacerFactor: 1.07, DrillWords: 45, BurstSecs: 15, ControlSecs: 30,
	}
}

func Dir() string {
	d, err := os.UserConfigDir()
	if err != nil {
		d = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(d, "sanda")
}

func path() string { return filepath.Join(Dir(), "config.toml") }

// Load returns the saved config, filling anything missing with defaults.
func Load() Config {
	c := Default()
	if _, err := toml.DecodeFile(path(), &c); err != nil && !os.IsNotExist(err) {
		return Default()
	}
	d := Default()
	if c.Time <= 0 {
		c.Time = d.Time
	}
	if c.Words <= 0 {
		c.Words = d.Words
	}
	if c.PacerFactor < 1 {
		c.PacerFactor = d.PacerFactor
	}
	if c.DrillWords <= 0 {
		c.DrillWords = d.DrillWords
	}
	if c.BurstSecs <= 0 {
		c.BurstSecs = d.BurstSecs
	}
	if c.ControlSecs <= 0 {
		c.ControlSecs = d.ControlSecs
	}
	switch c.Mode {
	case "time", "words", "quote":
	default:
		c.Mode = d.Mode
	}
	return c
}

func Save(c Config) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString("# sanda settings. Custom themes go in themes/<name>.toml next to this file.\n")
	if err := toml.NewEncoder(&buf).Encode(c); err != nil {
		return err
	}
	return os.WriteFile(path(), buf.Bytes(), 0o644)
}
