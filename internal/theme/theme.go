// Package theme holds sanda's colour themes. The schema matches Monkeytype's, so all of its themes work as presets.
package theme

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/lipgloss"

	"github.com/jinchiwei/sanda/internal/data"
)

// Theme uses Monkeytype's colour roles:
// bg background, main accent (stats, highlights), caret cursor, sub untyped text and secondary UI,
// subAlt panels, text correctly typed text, error wrong characters, errorExtra extra characters.
// bg = "terminal" draws no background at all, so the terminal's own background shows through.
type Theme struct {
	Name       string `json:"-" toml:"-"`
	Bg         string `json:"bg" toml:"bg"`
	Main       string `json:"main" toml:"main"`
	Caret      string `json:"caret" toml:"caret"`
	Sub        string `json:"sub" toml:"sub"`
	SubAlt     string `json:"subAlt" toml:"subAlt"`
	Text       string `json:"text" toml:"text"`
	Error      string `json:"error" toml:"error"`
	ErrorExtra string `json:"errorExtra" toml:"errorExtra"`
	// Accents are optional decorative colours: [0] warm (highlights, live wpm, raw line), [1] hot (labels, accuracy).
	// Themes without them (all Monkeytype presets) fall back to their own roles and stay as designed.
	Accents []string `json:"accents,omitempty" toml:"accents"`
}

// Builtin themes made for sanda, on top of the Monkeytype presets.
var builtin = map[string]Theme{
	"dreamtheme": {
		Bg: Terminal, Main: "#40E0D0", Caret: "#40E0D0", Sub: "#7a7a8c", SubAlt: "#16161d",
		Text: "#c8c8d0", Error: "#FF1493", ErrorExtra: "#8A2BE2",
		Accents: []string{"#F0C840", "#FF1493", "#BA55D3", "#FF7F50"}, // amber, deeppink, orchid, coral
	},
	"andromeda": {
		Bg: "#23262E", Main: "#00e8c6", Caret: "#FFE66D", Sub: "#746f77", SubAlt: "#2b2f38",
		Text: "#D5CED9", Error: "#ee5d43", ErrorExtra: "#ff00aa",
		Accents: []string{"#FFE66D", "#ff00aa", "#c74ded", "#7cb7ff"},
	},
}

const Default = "dreamtheme"

// Terminal as a bg value means "use the terminal's background".
const Terminal = "terminal"

var all map[string]Theme

// Load reads the presets plus any custom themes in ~/.config/sanda/themes/*.toml (custom wins on name clashes).
func Load() map[string]Theme {
	if all != nil {
		return all
	}
	all = map[string]Theme{}
	var mt map[string]Theme
	if err := json.Unmarshal(data.ThemesJSON(), &mt); err == nil {
		for k, t := range mt {
			all[k] = t
		}
	}
	for k, t := range builtin {
		all[k] = t
	}
	if dir, err := os.UserConfigDir(); err == nil {
		files, _ := filepath.Glob(filepath.Join(dir, "sanda", "themes", "*.toml"))
		for _, f := range files {
			var t Theme
			if _, err := toml.DecodeFile(f, &t); err == nil && t.Bg != "" {
				all[strings.TrimSuffix(filepath.Base(f), ".toml")] = t
			}
		}
	}
	for k, t := range all {
		t.Name = k
		t.normalize()
		all[k] = t
	}
	return all
}

// Names returns every theme name, sorted, with sanda's own themes first.
func Names() []string {
	m := Load()
	var names []string
	for k := range m {
		if _, ok := builtin[k]; !ok {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	return append([]string{"dreamtheme", "andromeda"}, names...)
}

// Get returns the named theme, or the default if it doesn't exist.
func Get(name string) Theme {
	m := Load()
	if t, ok := m[name]; ok {
		return t
	}
	return m[Default]
}

func (t *Theme) normalize() {
	if t.Bg != Terminal {
		t.Bg = hex6(t.Bg)
	}
	for _, p := range []*string{&t.Main, &t.Caret, &t.Sub, &t.SubAlt, &t.Text, &t.Error, &t.ErrorExtra} {
		*p = hex6(*p)
	}
	for i := range t.Accents {
		t.Accents[i] = hex6(t.Accents[i])
	}
}

// hex6 expands #rgb / #rgba and drops alpha from #rrggbbaa, since terminals have no alpha.
func hex6(h string) string {
	h = strings.TrimPrefix(strings.TrimSpace(h), "#")
	switch len(h) {
	case 3, 4:
		return "#" + string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	case 8:
		return "#" + h[:6]
	}
	return "#" + h
}

// Accent returns accent i, falling back to the theme's main, caret and error colours.
func (t Theme) Accent(i int) lipgloss.Color {
	fallback := []string{t.Main, t.Caret, t.Error}
	if i < len(t.Accents) {
		return lipgloss.Color(t.Accents[i])
	}
	return lipgloss.Color(fallback[i%len(fallback)])
}

// BgHex is the background colour, or "" when the terminal's own background is used.
func (t Theme) BgHex() string {
	if t.Bg == Terminal {
		return ""
	}
	return t.Bg
}

// OnCaret is the text colour for the character under the block caret.
func (t Theme) OnCaret() string {
	if t.Bg == Terminal {
		return "#111111"
	}
	return t.Bg
}
