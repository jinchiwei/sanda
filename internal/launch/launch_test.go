package launch

import (
	"strings"
	"testing"
)

func clearTermEnv(t *testing.T) {
	for _, k := range []string{"GHOSTTY_RESOURCES_DIR", "TERM_PROGRAM", "KITTY_WINDOW_ID", "WEZTERM_EXECUTABLE",
		"ALACRITTY_WINDOW_ID", "ALACRITTY_SOCKET"} {
		t.Setenv(k, "")
	}
	t.Setenv("TERM", "xterm-256color")
}

func TestDetect(t *testing.T) {
	cases := map[string][2]string{
		"ghostty":   {"GHOSTTY_RESOURCES_DIR", "/usr/share/ghostty"},
		"kitty":     {"KITTY_WINDOW_ID", "3"},
		"wezterm":   {"TERM_PROGRAM", "WezTerm"},
		"alacritty": {"ALACRITTY_WINDOW_ID", "1"},
		"foot":      {"TERM", "foot-extra"},
	}
	for want, kv := range cases {
		clearTermEnv(t)
		t.Setenv(kv[0], kv[1])
		if got := Detect(); got != want {
			t.Errorf("%s=%s: detected %q, want %q", kv[0], kv[1], got, want)
		}
	}
	clearTermEnv(t)
	if got := Detect(); got != "" {
		t.Errorf("plain xterm detected as %q", got)
	}
}

func TestCommandLines(t *testing.T) {
	cmd := []string{"/bin/sanda", "train"}
	want := map[string]string{
		"ghostty":   "--font-size=15.5 --class=io.github.jinchiwei.sanda --title=sanda --font-family=Geist Mono -e /bin/sanda train",
		"kitty":     "-o font_size=15.5 --class io.github.jinchiwei.sanda --title sanda -o font_family=Geist Mono /bin/sanda train",
		"alacritty": "-o font.size=15.5 --class io.github.jinchiwei.sanda --title sanda -o font.normal.family=\"Geist Mono\" -e /bin/sanda train",
		"wezterm":   "--config font_size=15.5 --config font=wezterm.font(\"Geist Mono\") start --class io.github.jinchiwei.sanda -- /bin/sanda train",
		"foot":      "--font Geist Mono:size=15.5 --app-id io.github.jinchiwei.sanda --title sanda /bin/sanda train",
	}
	for k, w := range want {
		if got := strings.Join(terminals[k].Args(15.5, "Geist Mono", cmd), " "); got != w {
			t.Errorf("%s:\n got  %s\n want %s", k, got, w)
		}
	}
}

func TestNoRelaunchInsideWindow(t *testing.T) {
	t.Setenv(InWindowEnv, "1")
	if _, err := Open(12, "", nil); err == nil {
		t.Fatal("a sanda window must never open another one")
	}
}
