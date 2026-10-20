# sanda (散打)

A terminal typing trainer that finds what actually slows you down and drills it.

Typing tests measure speed but don't train it, so practice on them tends to plateau. sanda keeps Monkeytype-style tests and also records every keystroke. From that it works out which transitions cost you time and builds practice around them:

- **Tests** like Monkeytype: time (15/30/60/120), words (10/25/50/100), quotes (short/medium/long/thicc, with the quote's source on the results screen), punctuation and numbers.
- **"What cost you"** on every result, in seconds. It covers Shift and capitals, punctuation, numbers, word starts, rarer words, corrections, slowing down, and your slowest letter pair.
- **Train**: adaptive drills built from real words that are dense in your costliest transitions. A transition leaves the list once it's as fast as your baseline, then comes back for review after 1, 2, 4, 8… days.
- **Pace**: overspeed bursts with a ghost cursor running at your recent median × 1.07, alternating with clean control rounds.
- **Progress**: the gap between your word-test and quote speed per week, transition types against your baseline, and your top weak spots with daily trends.
- **Themes**: sanda's `dreamtheme` and `andromeda`, plus all of Monkeytype's themes, with live preview. Add your own as TOML.

Train and pace rounds feed the weakness model but never your official WPM history.

## Install

Requires Go 1.26+ (the pure-Go SQLite driver needs it).

```sh
go install github.com/jinchiwei/sanda/cmd/sanda@latest
```

Or from a clone: `go build -o ~/.local/bin/sanda ./cmd/sanda`.

## Use

```text
sanda                  take a test with your saved settings
sanda -t 60            60-second test (flags are saved as your new defaults)
sanda -w 25 | -q long  word test / quote test
sanda -p -n            toggle punctuation / numbers
sanda train            adaptive drills
sanda pace             overspeed + control rounds
sanda progress         plateau gap, weak spots, trends
sanda themes           theme picker
sanda history [n]      last n official tests as plain text
sanda window [cmd]     same, in a new window of this terminal with a bigger font
```

### Bigger text: `sanda window`

A terminal program can't change its own font size, so `sanda window` opens a new window of the terminal you're in, with its own font size, running only sanda. Quitting sanda closes it. Supported: **Ghostty, kitty, Alacritty, WezTerm, foot**. Anywhere else (and on macOS for now) it just runs in place.

| Setting | Default | |
|---|---|---|
| `window` | `false` | make plain `sanda` always open a window |
| `window_font_size` | `0` (auto) | auto scales your current font by how much room the layout has, up to 2× |
| `window_font` | terminal's | font family for that window, e.g. `"Geist Mono"` |

The window's app id / class is `io.github.jinchiwei.sanda`, for window-manager rules. On Hyprland, for example:

```lua
o.window("^io\\.github\\.jinchiwei\\.sanda$", { float = true, center = true, size = { "(monitor_w*0.9)", "(monitor_h*0.9)" } })
```

Options (esc → options, or in the config):

| Option | Default | What it does |
|---|---|---|
| live wpm (`live_wpm`) | on | running WPM next to the timer |
| free backspace (`free_backspace`) | on | backspace into any earlier word, not only wrong ones (Monkeytype's "freedom mode") |
| terminal bg (`terminal_background`) | off | use your terminal's background with any theme (dreamtheme always does) |

In the menu, **j/k** (or ↓/↑) move between rows and **h/l** (or ←/→) move within a row.

Keys: **tab** restart / next, **esc** menu, **ctrl+backspace** or **ctrl+w** delete word, **ctrl+c** quit.

## Files

| Path | What |
|---|---|
| `~/.config/sanda/config.toml` | settings (theme, test mode, pacer factor, round lengths) |
| `~/.config/sanda/themes/<name>.toml` | custom themes |
| `~/.local/share/sanda/sanda.db` | every test and keystroke (SQLite) |

A custom theme uses Monkeytype's colour roles:

```toml
bg = "#0a0a0a"         # background, or "terminal" to use the terminal's own
main = "#40E0D0"       # accent: stats, highlights
caret = "#40E0D0"      # cursor
text = "#c8c8d0"       # typed text
sub = "#7a7a8c"        # text not typed yet, secondary UI
subAlt = "#16161d"     # panels
error = "#FF1493"      # wrong characters
errorExtra = "#8A2BE2" # extra characters
accents = ["#F0C840", "#BA55D3", "#FF7F50"]  # optional
```

## How the analysis works

Every keystroke is logged with its timestamp, the character expected and whether it was right on the first try.

- A **transition** is a clean move from one character to the next: both keys correct on the first try, with no pause over 1.5 s. Its time is the key-to-key interval.
- Each transition's cost is estimated as:

  `how often it occurs in English quotes × (your median for it − your overall median) + error rate × (the time an error costs)`

  "The time an error costs" is about four keystrokes. The result is time lost per 1000 characters of typical text, and the drill list is ranked by it.

Key timestamps are taken when the terminal delivers the event. Terminal latency adds a roughly constant offset, so the numbers are good for comparing transitions with each other, not as absolute reaction times.

See [docs/spec.md](docs/spec.md) for the design.

## Credits

Quotes, word lists and preset themes come from [Monkeytype](https://github.com/monkeytypegame/monkeytype) (GPL-3.0); see [internal/data/assets/ATTRIBUTION.md](internal/data/assets/ATTRIBUTION.md).

## License

GPL-3.0. See [LICENSE](LICENSE).
