// Package launch opens sanda in a new window of the terminal it was started from, with its own font size.
//
// A terminal program can't change its own font size, but most terminals accept a font size when
// starting a new window. sanda detects the current terminal from the environment, picks a size and
// starts a new window that runs only sanda, so quitting sanda closes it. Unknown terminals (or macOS,
// where terminals launch differently) fall back to running in place.
package launch

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// InWindowEnv marks a process already running in a sanda window, so it never relaunches itself.
const InWindowEnv = "SANDA_WINDOW"

// AppID is the window class/app id of sanda windows, for window-manager rules.
const AppID = "io.github.jinchiwei.sanda"

// Terminal knows how to open a new window of one terminal emulator.
type Terminal struct {
	Name        string
	Binary      string
	DefaultSize float64 // font size if the current one can't be read
	// Args builds the command line for a new window running cmd with the given font size and family.
	Args func(size float64, family string, cmd []string) []string
}

var terminals = map[string]Terminal{
	"ghostty": {"Ghostty", "ghostty", 12, func(size float64, family string, cmd []string) []string {
		a := []string{fmt.Sprintf("--font-size=%g", size), "--class=" + AppID, "--title=sanda"}
		if family != "" {
			a = append(a, "--font-family="+family)
		}
		return append(append(a, "-e"), cmd...)
	}},
	"kitty": {"kitty", "kitty", 11, func(size float64, family string, cmd []string) []string {
		a := []string{"-o", fmt.Sprintf("font_size=%g", size), "--class", AppID, "--title", "sanda"}
		if family != "" {
			a = append(a, "-o", "font_family="+family)
		}
		return append(a, cmd...)
	}},
	"alacritty": {"Alacritty", "alacritty", 11.25, func(size float64, family string, cmd []string) []string {
		a := []string{"-o", fmt.Sprintf("font.size=%g", size), "--class", AppID, "--title", "sanda"}
		if family != "" {
			a = append(a, "-o", fmt.Sprintf("font.normal.family=%q", family))
		}
		return append(append(a, "-e"), cmd...)
	}},
	"wezterm": {"WezTerm", "wezterm", 12, func(size float64, family string, cmd []string) []string {
		a := []string{"--config", fmt.Sprintf("font_size=%g", size)}
		if family != "" {
			a = append(a, "--config", fmt.Sprintf("font=wezterm.font(%q)", family))
		}
		return append(append(a, "start", "--class", AppID, "--"), cmd...)
	}},
	"foot": {"foot", "foot", 8, func(size float64, family string, cmd []string) []string {
		if family == "" {
			family = "monospace"
		}
		return append([]string{"--font", fmt.Sprintf("%s:size=%g", family, size), "--app-id", AppID, "--title", "sanda"}, cmd...)
	}},
}

// Detect works out which terminal sanda is running in from the variables terminals set ("" if unknown).
func Detect() string {
	env := os.Getenv
	switch {
	case env("GHOSTTY_RESOURCES_DIR") != "" || env("TERM_PROGRAM") == "ghostty":
		return "ghostty"
	case env("KITTY_WINDOW_ID") != "" || env("TERM") == "xterm-kitty":
		return "kitty"
	case env("WEZTERM_EXECUTABLE") != "" || env("TERM_PROGRAM") == "WezTerm":
		return "wezterm"
	case env("ALACRITTY_WINDOW_ID") != "" || env("ALACRITTY_SOCKET") != "":
		return "alacritty"
	case strings.HasPrefix(env("TERM"), "foot"):
		return "foot"
	}
	return ""
}

// currentFontSize reads the running terminal's font size where that's possible (Ghostty), else its default.
func currentFontSize(key string) float64 {
	t := terminals[key]
	if key == "ghostty" {
		if out, err := exec.Command(t.Binary, "+show-config").Output(); err == nil {
			if m := regexp.MustCompile(`(?m)^font-size = ([0-9.]+)`).FindSubmatch(out); m != nil {
				if v, err := strconv.ParseFloat(string(m[1]), 64); err == nil {
					return v
				}
			}
		}
	}
	return t.DefaultSize
}

// Terminal size sanda needs for the test with the small logo above it.
const needCols, needRows = 92, 38

// AutoSize scales the current font by how much larger sanda's layout could be in a window the size of
// the current one, between 1× and 2×.
func AutoSize(key string) float64 {
	cur := currentFontSize(key)
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 || ws.Row == 0 {
		return math.Round(cur*1.5*2) / 2
	}
	f := math.Min(float64(ws.Col)/needCols, float64(ws.Row)/needRows)
	f = math.Max(1, math.Min(2, f))
	return math.Round(cur*f*2) / 2 // to the nearest half point
}

// Open starts sanda (with args) in a new window. It returns the terminal's display name, or an error
// explaining why it has to run in place instead.
func Open(fontSize float64, family string, args []string) (string, error) {
	if os.Getenv(InWindowEnv) != "" {
		return "", fmt.Errorf("already in a sanda window")
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" {
		return "", fmt.Errorf("separate windows are only supported on Linux/BSD for now")
	}
	key := Detect()
	if key == "" {
		return "", fmt.Errorf("this terminal can't be given a font size from the command line (supported: Ghostty, kitty, Alacritty, WezTerm, foot)")
	}
	t := terminals[key]
	bin, err := exec.LookPath(t.Binary)
	if err != nil {
		return "", fmt.Errorf("%s isn't on PATH", t.Binary)
	}
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	if fontSize <= 0 {
		fontSize = AutoSize(key)
	}
	cmd := exec.Command(bin, t.Args(fontSize, family, append([]string{self}, args...))...)
	cmd.Env = append(os.Environ(), InWindowEnv+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // survives this shell closing
	if err := cmd.Start(); err != nil {
		return "", err
	}
	cmd.Process.Release()
	return fmt.Sprintf("%s (font size %g)", t.Name, fontSize), nil
}
