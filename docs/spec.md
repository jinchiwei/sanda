# sanda (散打) — spec

Status: **M1–M4 implemented** (2026-10-20). See the milestone table for notes.

A terminal typing trainer that measures exactly what slows you down and drills that, instead of only testing you.
The name: 散打 is wushu free sparring; 打 is also the "da" in 打字 (typing).

## 1. The problem

Typing tests (Monkeytype, couik) *measure* speed but don't *train* it. Practising on them reinforces the same habits, so speed plateaus.
The author has been flat for 3–4 years at **~130 WPM on quotes** and **~170 WPM on 15 s words**.

That 40 WPM gap is the target. Quotes add things a 15 s word test doesn't have:

- capitals (Shift + letter transitions)
- punctuation and numbers
- longer and rarer words
- holding pace over a longer text (fatigue / drift)

sanda's job: find out which of these costs the most, drill it, and prove it improved.

## 2. Principles

1. **Measure at the keystroke level.** WPM hides the cause. Every keystroke is timed, and transitions between characters are scored.
2. **Target the costliest weaknesses.** Practice is generated from your weakest transitions, weighted by how often they occur in real text.
3. **Push slightly past comfort.** A pacer runs a little faster than you in short bursts, then you consolidate at an accurate pace.
4. **Verify over time.** Weak items come back on a spaced schedule. Progress is shown per weakness, not just as WPM.

## 3. Modes

### 3.1 Test (Monkeytype-style)
- **time:** 15 / 30 / 60 / 120 s
- **words:** 10 / 25 / 50 / 100
- **quote:** short / medium / long / thicc (Monkeytype's length groups)
- toggles: punctuation, numbers

### 3.2 Train (adaptive drills)
Short rounds of about 30 s built from your current weaknesses (see §5). Each round names its focus, e.g. "Shift transitions: `T`, `I`, `W`" or "slow pairs: `ce` `xp` `nk`".

### 3.3 Pace (overspeed)
- A ghost caret moves at a target speed, by default your recent median for that text type × 1.07.
- Rounds alternate:
  - **burst:** 15 s at the pacer
  - **control:** 30 s with no pacer, scored on accuracy

## 4. Results screen (Monkeytype-style)

- **Headline:** WPM, accuracy.
- **Secondary:** raw WPM, consistency, characters (correct / incorrect / extra / missed), time.
- **WPM-over-time chart**, with error markers.
- **Quote source**, shown under the stats for quote tests (e.g. "— Meditations to Heal Your Life"). Monkeytype's quote data includes a source for every quote.
- **"What cost you"** (sanda-specific): the top 3 losses this test, in estimated seconds. Example:
  - "Shift + letter: −2.1 s"
  - "`,` → space → letter: −1.4 s"
  - "pace drift after 20 s: −1.0 s"

## 5. Measurement and the weakness model

### Raw events (per keystroke)
- monotonic timestamp (ns)
- expected char
- typed char
- correct?
- backspace?
- position in word / text

### Derived per test
- **inter-key interval (IKI)** for each correct transition `a→b`
- **first-try errors** per char and per transition
- **hesitations:** IKI > 2× your median
- **word timings**
- **pace curve** across the test

### Units tracked
- bigrams and trigrams
- character classes: Shift + letter, punctuation, digits, space after punctuation
- whole words

### Score per unit
An estimate of how much time the unit costs you over real text:

```
cost(unit) = freq_in_corpus(unit) × max(0, median_IKI(unit) − baseline_IKI)
           + error_rate(unit) × error_penalty
```

- `baseline_IKI` is your median over all transitions.
- Units with too few samples are marked "uncertain" and get probed rather than drilled.

### Graduation
A unit drops off the drill list once its median IKI is within baseline across N samples on at least 2 different days.

## 6. Drill generation

1. Take the top-k weaknesses by cost.
2. Pick words from the 25k English list that contain them, weighted toward common words.
3. For class weaknesses, apply the class: add capitals (Shift), punctuation, or digits in realistic positions.
4. Mix in about 30% filler of strong material so rhythm stays natural.
5. Optionally, pull real quote sentences that are dense in the weak units.

## 7. Progress view

- Top weaknesses over time, as a sparkline per unit.
- Quote-vs-words gap over time, the headline plateau metric.
- Weekly summary: units graduated, new weaknesses found.

## 8. Themes

- The theme format matches Monkeytype's colour schema: `bg`, `main`, `caret`, `sub`, `subAlt`, `text`, `error`, `errorExtra`, `colorfulError`, `colorfulErrorExtra`.
- **Presets:** Monkeytype's theme list, about 188 themes, imported. Plus:
  - **dreamtheme**:

    | key | colour | used for (Monkeytype meaning) |
    |---|---|---|
    | bg | `terminal` | use terminal background |
    | main | `#FF1493` deeppink | accent: stats, highlights, active selection |
    | caret | `#40E0D0` turquoise | cursor |
    | text | `#F5F5FA` | general UI text |
    | typed | `#F0C840` | correctly typed text (optional, defaults to text) |
    | untyped | `#F5F5FA` | not-yet-typed text (optional, defaults to sub) |
    | sub | `#7a7a8c` | secondary UI |
    | subAlt | `#16161d` | panels / key-hint backgrounds |
    | error | `#BF40FF` neon violet | wrong character |
    | errorExtra | `#FF7F50` coral | extra characters past the word |

  - **andromeda**:

    | key | colour |
    |---|---|
    | bg | `#23262E` |
    | main | `#00e8c6` |
    | caret | `#FFE66D` |
    | text | `#FFE66D` |
    | error | `#ee5d43` |
    | errorExtra | `#ff00aa` |

- Optional `typed` overrides writing colour; presets without it fall back to `text`.
- Optional `untyped` overrides pending text colour; presets without it fall back to `sub`.
- Truecolor output. A theme picker with live preview. Custom themes go in `~/.config/sanda/themes/*.toml`.
- The font is the terminal's (a terminal app can't set its own).

## 9. Data, config, storage

- **Text data:** Monkeytype's English quotes and word lists (1k / 5k / 10k / 25k), vendored in `data/` with attribution. Monkeytype is GPL-3.0, so sanda is **GPL-3.0**.
- **Config:** `~/.config/sanda/config.toml`, covering theme, default mode, pacer factor and similar settings.
- **Storage:** `~/.local/share/sanda/sanda.db`, SQLite via pure-Go `modernc.org/sqlite`.
  - It holds tests, keystroke events and unit stats.
  - Raw events are kept so the model can be recomputed if the scoring changes.

## 10. Tech

- Go, with Bubble Tea (TUI), Lip Gloss (styling) and an ntcharts-style chart for the WPM graph.
- Go installed via mise (no sudo).
- Single binary: `sanda`.
- Timing note: key timestamps are taken when the event arrives in the program, so terminal latency adds a roughly constant offset. That's fine for comparing transitions against each other, but not for absolute ms claims.

## 11. Milestones

| # | Scope | Done when |
|---|---|---|
| M1 | Test modes, results screen with quote source, themes (presets + dreamtheme + andromeda), keystroke logging to SQLite | Can replace Monkeytype for daily tests |
| M2 | Analysis: per-test "what cost you", unit stats, weakness list | Shows *why* quotes are 40 WPM slower |
| M3 | Train mode: adaptive drill generation, graduation | Drills target the top weaknesses |
| M4 | Pace mode, spaced repetition, progress view | Plateau metric visible over weeks |

### Implementation notes (2026-10-20)
- All four milestones are built. Tested with unit tests (engine scoring, transitions, breakdown, model ranking, drill generation) and an end-to-end run in tmux: tests, quotes with source, menu, train with schedule updates, pace burst/control with the ghost, progress, and the theme picker.
- The weakness model uses the last 90 days, and each transition's median uses its 60 most recent samples. Transitions with fewer than 6 samples are probed rather than ranked.
- Abandoned tests (tab, esc or quit mid-test) are not saved.
- Before any history exists, train drills the three most common English letter pairs as probes.

## 12. Not in scope (for now)

Multiplayer, ghost replays, a web version, languages other than English.

## 13. Open questions

Decided:
- Abandoned tests are not saved.
- Train and Pace rounds are **kept separate** from the official WPM history. Their keystrokes still feed the weakness model.
- dreamtheme: cursor turquoise. Untyped text is `untyped` white, secondary labels are `sub` grey, typed text is `typed` amber.
