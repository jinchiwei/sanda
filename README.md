# sanda (散打)

A terminal typing trainer that finds what actually slows you down and drills it.

Typing tests measure speed but don't train it, so practice on them tends to plateau. sanda keeps Monkeytype-style tests and also records every keystroke. From that it works out which transitions cost you time and builds practice around them:

- **Tests** like Monkeytype: time (15/30/60/120), words (10/25/50/100), quotes (short/medium/long/thicc, with the quote's source on the results screen), punctuation and numbers. Time and words results show your personal bests for every length, Monkeytype style; quote results show your record on that quote.
- **"What cost you"** on every result, in seconds. It covers Shift and capitals, punctuation, numbers, word starts, rarer words, corrections, slowing down, and your slowest letter pair.
- **Train**: adaptive drills built from real words that are dense in your costliest transitions. Train follows the test format you're set to: for time and words tests it drills lowercase words only and learns only from those tests; for quotes (or punctuation on) it learns from quotes and also drills capitals and punctuation. Each transition is held to *par*, your median for that kind of transition in that format: word starts against word starts, letter pairs against letter pairs, a capital against the same lowercase move plus 40% for Shift. A transition leaves the list once it's within 5% of par, then comes back for review after 1, 2, 4, 8… days; a miss rests it 20 minutes so other weak spots get their turn. Capitals are drilled in rounds of their own, as short sentences, because they slow everything typed around them.
- **Pace**: overspeed bursts with a ghost cursor running at your recent median × 1.07, alternating with clean control rounds.
- **Progress**: the gap between your word-test and quote speed per week, transition types against your baseline, and your top weak spots (for the format you're set to) with their par and daily trends.
- **Themes**: sanda's `dreamtheme` and `andromeda`, plus all of Monkeytype's themes, with live preview. Add your own as TOML.

Train rounds feed the weakness model but never your official WPM history.

## Install

Works on Linux, macOS and Windows (CI runs the tests on all three). On Windows use Windows Terminal (or another modern terminal) for colours and the block-character logo; `sanda window` is Linux/BSD only and runs in place elsewhere. Data lives in `%LocalAppData%\sanda` on Windows.

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
sanda freedom          free writing with live WPM; nothing saved
sanda train            adaptive drills
sanda pace             overspeed + control rounds
sanda progress         plateau gap, weak spots, trends
sanda themes           theme picker
sanda history [n]      last n official tests as plain text (in the app: esc → history, enter reopens a result)
sanda window [cmd]     same, in a new window of this terminal with a bigger font
```

### Freedom mode

Run `sanda freedom`, or choose **freedom** from the menu, for a blank writing area.
Type whatever you like; Enter starts a new line and backspace edits your writing.
Live WPM counts the characters currently in your text (five characters per word),
including spaces and line breaks, divided by the time since you started typing.
There is no timer, correctness scoring, or results screen. Text stays only in memory:
no tests, keystrokes, personal bests, or training records are saved. Tab clears the text
and resets WPM; Esc opens the menu (returning starts fresh); Ctrl+C quits.

The 散打 logo follows each theme: caret for 散, main for 打, and typed for highlights.
Dreamtheme uses turquoise, deeppink, and amber respectively. WPM uses turquoise;
progress highlights use deeppink.

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
| terminal bg (`terminal_background`) | off | use your terminal's background with any theme (dreamtheme uses this by default) |
| keep logo (`keep_logo`) | on | keep the 散打 logo on screen while typing (off = it dims away in 150 ms when typing starts) |

In the menu, **j/k** (or ↓/↑) move between rows and **h/l** (or ←/→) move within a row.

Keys: **tab** restart / next, **esc** menu, **ctrl+backspace** or **ctrl+w** delete word, **ctrl+c** quit.
On the results screen, **r** retakes the same text and settings; **tab** starts the next test.
Completed retakes are saved as separate attempts.
Quote tests also show your record on that specific quote: attempt number, your best on it, and ★ when you beat it (retakes count; history shows the record as it stood at each attempt).

Incorrect letters stay highlighted in the prompt, with the actual character you typed
shown directly beneath them. Pressing Space before finishing a word shows `␣`
beneath the first skipped letter. Correcting the character removes its hint.

## Files

| Path | What |
|---|---|
| `~/.config/sanda/config.toml` | settings (theme, test mode, pacer factor, round lengths) |
| `~/.config/sanda/themes/<name>.toml` | custom themes |
| `~/.local/share/sanda/sanda.db` | every test and keystroke (SQLite) |

A custom theme uses Monkeytype's colour roles:

```toml
bg = "#0a0a0a"         # background, or "terminal" to use the terminal's own
main = "#FF1493"       # accent: stats, highlights
caret = "#40E0D0"      # cursor
text = "#F5F5FA"       # general UI text
typed = "#F0C840"      # correctly typed text; optional, falls back to text
untyped = "#F5F5FA"    # text not typed yet; optional, falls back to sub
sub = "#7a7a8c"        # secondary UI; default untyped color
subAlt = "#16161d"     # panels
error = "#BF40FF"      # wrong characters
errorExtra = "#FF7F50" # extra characters
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
