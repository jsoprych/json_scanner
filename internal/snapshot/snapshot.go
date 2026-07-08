// Package snapshot materializes the cross-sectional feature snapshot into a SQLite
// table so studies can be plain SQL WHERE/ORDER BY over it — SQL is the study
// language, no DSL. The table is tiny (one row per symbol), so an in-memory DB is
// instant; an on-disk path lets you also query it by hand with the sqlite3 CLI.
package snapshot

import (
	"database/sql"
	"fmt"
	"math"
	"strings"

	"cetus-marketdata-scanner/internal/screen"
	"cetus-marketdata-scanner/internal/study"

	_ "modernc.org/sqlite"
)

// DB is a snapshot store.
type DB struct{ db *sql.DB }

// Open opens a snapshot DB; path "" (or ":memory:") uses an in-memory DB. A single
// connection is pinned so an in-memory table survives for the DB's lifetime.
func Open(path string) (*DB, error) {
	dsn := ":memory:"
	if path != "" && path != ":memory:" {
		dsn = "file:" + path
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return &DB{db: db}, nil
}

// Close releases the DB.
func (d *DB) Close() error { return d.db.Close() }

// columns of the snapshot table, in insert order.
var columns = []string{
	"symbol", "timestamp", "close", "prev_close", "high", "low", "dollar_vol",
	"sma50", "sma200", "prev_sma50", "prev_sma200",
	"rsi14", "prev_rsi14", "ret_3m", "high_52w", "low_52w",
}

// Load (re)creates the snapshot table and inserts rows. NaN → NULL so SQL
// comparisons behave: an under-warmed feature is NULL and simply never matches.
func (d *DB) Load(rows []screen.SnapshotRow, ts int64) error {
	if _, err := d.db.Exec(`DROP TABLE IF EXISTS snapshot`); err != nil {
		return fmt.Errorf("drop snapshot: %w", err)
	}
	if _, err := d.db.Exec(`CREATE TABLE snapshot(
		symbol TEXT PRIMARY KEY, timestamp INTEGER,
		close REAL, prev_close REAL, high REAL, low REAL, dollar_vol REAL,
		sma50 REAL, sma200 REAL, prev_sma50 REAL, prev_sma200 REAL,
		rsi14 REAL, prev_rsi14 REAL, ret_3m REAL, high_52w REAL, low_52w REAL)`); err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",")
	stmt, err := tx.Prepare("INSERT INTO snapshot(" + strings.Join(columns, ",") + ") VALUES(" + ph + ")")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(r.Symbol, ts,
			nz(r.Close), nz(r.PrevClose), nz(r.High), nz(r.Low), nz(r.DollarVol),
			nz(r.SMA50), nz(r.SMA200), nz(r.PrevSMA50), nz(r.PrevSMA200),
			nz(r.RSI14), nz(r.PrevRSI14), nz(r.Ret3m), nz(r.High52w), nz(r.Low52w)); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// nz maps NaN → nil (SQL NULL).
func nz(v float64) any {
	if math.IsNaN(v) {
		return nil
	}
	return v
}

// Match is one study hit (fixed display projection).
type Match struct {
	Symbol    string  `json:"symbol"`
	Close     float64 `json:"close"`
	RSI14     float64 `json:"rsi14"`
	Ret3m     float64 `json:"ret_3m"`
	DollarVol float64 `json:"dollar_vol"`
}

// Run executes a study's WHERE / ORDER BY / LIMIT and returns matches.
//
// The study SQL is TRUSTED — it comes from the Global user's local config. When
// untrusted, user-authored SQL arrives (multi-user, paid tier), it must be
// validated/sandboxed (allow-list of columns/operators) before reaching here.
func (d *DB) Run(s study.Study) ([]Match, error) {
	where := strings.TrimSpace(s.Where)
	if where == "" {
		where = "1=1"
	}
	q := "SELECT symbol, close, rsi14, ret_3m, dollar_vol FROM snapshot WHERE (" + where + ")"
	if s.OrderBy != "" {
		q += " ORDER BY " + s.OrderBy
	}
	if s.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", s.Limit)
	}
	rows, err := d.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("study %q: %w", s.Key, err)
	}
	defer rows.Close()

	var out []Match
	for rows.Next() {
		var m Match
		var c, rs, rt, dv sql.NullFloat64
		if err := rows.Scan(&m.Symbol, &c, &rs, &rt, &dv); err != nil {
			return nil, err
		}
		m.Close, m.RSI14, m.Ret3m, m.DollarVol = c.Float64, rs.Float64, rt.Float64, dv.Float64
		out = append(out, m)
	}
	return out, rows.Err()
}
