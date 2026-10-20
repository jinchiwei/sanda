// Package store keeps tests and their raw keystrokes in SQLite (~/.local/share/sanda/sanda.db).
package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/jinchiwei/sanda/internal/engine"
)

// Kinds of saved rounds. Only KindOfficial counts toward WPM history and personal bests.
const (
	KindOfficial = "official"
	KindTrain    = "train"
	KindPace     = "pace"
)

type Test struct {
	ID          int64
	StartedAt   time.Time
	Kind        string
	Mode        string // time, words, quote, drill, burst, control
	ModeValue   string // 30, 50, short, …
	Punctuation bool
	Numbers     bool
	QuoteID     int
	Text        string // the words, space-separated
	Focus       string // train/pace: the units drilled, comma-separated
	WPM, Raw    float64
	Acc, Cons   float64
	DurationMs  int64
	Correct     int
	Incorrect   int
	Extra       int
	Missed      int
}

type DB struct{ db *sql.DB }

func DefaultPath() string {
	d := os.Getenv("XDG_DATA_HOME")
	if d == "" {
		d = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}
	return filepath.Join(d, "sanda", "sanda.db")
}

const schema = `
CREATE TABLE IF NOT EXISTS tests (
  id INTEGER PRIMARY KEY,
  started_at TEXT NOT NULL,
  kind TEXT NOT NULL,
  mode TEXT NOT NULL,
  mode_value TEXT NOT NULL,
  punctuation INTEGER NOT NULL DEFAULT 0,
  numbers INTEGER NOT NULL DEFAULT 0,
  quote_id INTEGER NOT NULL DEFAULT 0,
  text TEXT NOT NULL,
  focus TEXT NOT NULL DEFAULT '',
  wpm REAL, raw REAL, acc REAL, consistency REAL,
  duration_ms INTEGER,
  chars_correct INTEGER, chars_incorrect INTEGER, chars_extra INTEGER, chars_missed INTEGER
);
CREATE INDEX IF NOT EXISTS tests_kind_time ON tests(kind, started_at);
CREATE TABLE IF NOT EXISTS events (
  test_id INTEGER NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
  seq INTEGER NOT NULL,
  t_us INTEGER NOT NULL,
  kind INTEGER NOT NULL,
  typed INTEGER NOT NULL,
  expected INTEGER NOT NULL,
  correct INTEGER NOT NULL,
  first INTEGER NOT NULL,
  word INTEGER NOT NULL,
  pos INTEGER NOT NULL,
  PRIMARY KEY (test_id, seq)
);
CREATE TABLE IF NOT EXISTS schedule (
  unit TEXT PRIMARY KEY,
  level INTEGER NOT NULL,
  due TEXT NOT NULL,
  last_seen TEXT NOT NULL
);
`

func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(3000)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() error { return d.db.Close() }

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// SaveTest stores a finished round and its keystrokes.
func (d *DB) SaveTest(t Test, events []engine.Event) (int64, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO tests (started_at, kind, mode, mode_value, punctuation, numbers, quote_id, text, focus,
		wpm, raw, acc, consistency, duration_ms, chars_correct, chars_incorrect, chars_extra, chars_missed)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.StartedAt.UTC().Format(time.RFC3339Nano), t.Kind, t.Mode, t.ModeValue, b2i(t.Punctuation), b2i(t.Numbers),
		t.QuoteID, t.Text, t.Focus, t.WPM, t.Raw, t.Acc, t.Cons, t.DurationMs, t.Correct, t.Incorrect, t.Extra, t.Missed)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	stmt, err := tx.Prepare(`INSERT INTO events (test_id, seq, t_us, kind, typed, expected, correct, first, word, pos)
		VALUES (?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for i, e := range events {
		if _, err := stmt.Exec(id, i, e.T.Microseconds(), int(e.Kind), int(e.Typed), int(e.Expected),
			b2i(e.Correct), b2i(e.First), e.Word, e.Pos); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

const testCols = `id, started_at, kind, mode, mode_value, punctuation, numbers, quote_id, text, focus,
	wpm, raw, acc, consistency, duration_ms, chars_correct, chars_incorrect, chars_extra, chars_missed`

func scanTest(r interface{ Scan(...any) error }) (Test, error) {
	var t Test
	var at string
	var p, n int
	err := r.Scan(&t.ID, &at, &t.Kind, &t.Mode, &t.ModeValue, &p, &n, &t.QuoteID, &t.Text, &t.Focus,
		&t.WPM, &t.Raw, &t.Acc, &t.Cons, &t.DurationMs, &t.Correct, &t.Incorrect, &t.Extra, &t.Missed)
	t.StartedAt, _ = time.Parse(time.RFC3339Nano, at)
	t.Punctuation, t.Numbers = p == 1, n == 1
	return t, err
}

// Tests returns rounds of the given kinds since a time, oldest first.
func (d *DB) Tests(since time.Time, kinds ...string) ([]Test, error) {
	q := `SELECT ` + testCols + ` FROM tests WHERE started_at >= ?`
	args := []any{since.UTC().Format(time.RFC3339Nano)}
	if len(kinds) > 0 {
		q += ` AND kind IN (?` + strings.Repeat(",?", len(kinds)-1) + `)`
		for _, k := range kinds {
			args = append(args, k)
		}
	}
	rows, err := d.db.Query(q+` ORDER BY started_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Test
	for rows.Next() {
		t, err := scanTest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// BestWPM is the personal best for the same official test settings (0 if none).
func (d *DB) BestWPM(mode, value string, punct, numbers bool) float64 {
	var best sql.NullFloat64
	d.db.QueryRow(`SELECT MAX(wpm) FROM tests WHERE kind = ? AND mode = ? AND mode_value = ? AND punctuation = ? AND numbers = ?`,
		KindOfficial, mode, value, b2i(punct), b2i(numbers)).Scan(&best)
	return best.Float64
}

// TestEvents is one round with its keystrokes, for analysis.
type TestEvents struct {
	Test   Test
	Events []engine.Event
}

// Events loads every round since a time with its keystrokes, oldest first.
func (d *DB) Events(since time.Time) ([]TestEvents, error) {
	tests, err := d.Tests(since)
	if err != nil {
		return nil, err
	}
	idx := map[int64]int{}
	out := make([]TestEvents, len(tests))
	for i, t := range tests {
		out[i].Test = t
		idx[t.ID] = i
	}
	rows, err := d.db.Query(`SELECT e.test_id, e.t_us, e.kind, e.typed, e.expected, e.correct, e.first, e.word, e.pos
		FROM events e JOIN tests t ON t.id = e.test_id WHERE t.started_at >= ? ORDER BY e.test_id, e.seq`,
		since.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, tus int64
		var kind, typed, exp, correct, first, word, pos int
		if err := rows.Scan(&id, &tus, &kind, &typed, &exp, &correct, &first, &word, &pos); err != nil {
			return nil, err
		}
		i, ok := idx[id]
		if !ok {
			continue
		}
		out[i].Events = append(out[i].Events, engine.Event{
			T: time.Duration(tus) * time.Microsecond, Kind: engine.Kind(kind), Typed: rune(typed), Expected: rune(exp),
			Correct: correct == 1, First: first == 1, Word: word, Pos: pos,
		})
	}
	return out, rows.Err()
}

// Sched is a unit's spaced-repetition state.
type Sched struct {
	Unit     string
	Level    int
	Due      time.Time
	LastSeen time.Time
}

func (d *DB) Schedule() (map[string]Sched, error) {
	rows, err := d.db.Query(`SELECT unit, level, due, last_seen FROM schedule`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Sched{}
	for rows.Next() {
		var s Sched
		var due, seen string
		if err := rows.Scan(&s.Unit, &s.Level, &due, &seen); err != nil {
			return nil, err
		}
		s.Due, _ = time.Parse(time.RFC3339, due)
		s.LastSeen, _ = time.Parse(time.RFC3339, seen)
		out[s.Unit] = s
	}
	return out, rows.Err()
}

func (d *DB) SetSchedule(s Sched) error {
	_, err := d.db.Exec(`INSERT INTO schedule (unit, level, due, last_seen) VALUES (?,?,?,?)
		ON CONFLICT(unit) DO UPDATE SET level = excluded.level, due = excluded.due, last_seen = excluded.last_seen`,
		s.Unit, s.Level, s.Due.UTC().Format(time.RFC3339), s.LastSeen.UTC().Format(time.RFC3339))
	return err
}
