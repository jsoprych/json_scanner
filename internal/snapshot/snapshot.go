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
type DB struct {
	db           *sql.DB
	snapshotID   string
	symbolCount  int
	snapshotDate int64
}

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
	"symbol", "timestamp", "close", "high", "low", "open",
	// Trend indicators
	"sma5", "sma10", "sma20", "sma30", "sma50", "sma100", "sma200",
	"ema10", "ema21", "ema50", "ema100", "ema200",
	"pct_from_sma50", "pct_from_sma200", "ma_stack",
	// Momentum indicators
	"rsi14", "macd", "macd_signal", "macd_hist",
	"stoch_k", "stoch_d", "willr14", "cci20",
	"roc10", "roc20", "adx14", "di_plus", "di_minus",
	// Volatility indicators
	"atr14", "atr_pct",
	"bb_upper", "bb_mid", "bb_lower", "bb_bandwidth", "bb_pct_b",
	"hist_vol20",
	// Price structure
	"high_52w", "low_52w", "is_52w_high", "is_52w_low",
	"gap_pct", "true_range", "pct_off_52w_high", "pct_above_52w_low",
	// Returns
	"ret_1d", "ret_5d", "ret_1m", "ret_3m", "ret_6m", "ret_1y",
	// Volume indicators
	"dollar_vol", "avg_dollar_vol20", "rel_volume", "obv", "vwap_dist", "mfi14",
	// Cross-detection booleans
	"golden_cross", "oversold_bounce",
}

// Load (re)creates the snapshot table and inserts rows. NaN → NULL so SQL
// comparisons behave: an under-warmed feature is NULL and simply never matches.
func (d *DB) Load(rows []screen.SnapshotRow, ts int64) error {
	if _, err := d.db.Exec(`DROP TABLE IF EXISTS snapshot`); err != nil {
		return fmt.Errorf("drop snapshot: %w", err)
	}
	if _, err := d.db.Exec(`CREATE TABLE snapshot(
		symbol TEXT PRIMARY KEY, timestamp INTEGER,
		close REAL, high REAL, low REAL, open REAL,
		sma5 REAL, sma10 REAL, sma20 REAL, sma30 REAL, sma50 REAL, sma100 REAL, sma200 REAL,
		ema10 REAL, ema21 REAL, ema50 REAL, ema100 REAL, ema200 REAL,
		pct_from_sma50 REAL, pct_from_sma200 REAL, ma_stack INTEGER,
		rsi14 REAL, macd REAL, macd_signal REAL, macd_hist REAL,
		stoch_k REAL, stoch_d REAL, willr14 REAL, cci20 REAL,
		roc10 REAL, roc20 REAL, adx14 REAL, di_plus REAL, di_minus REAL,
		atr14 REAL, atr_pct REAL,
		bb_upper REAL, bb_mid REAL, bb_lower REAL, bb_bandwidth REAL, bb_pct_b REAL,
		hist_vol20 REAL,
		high_52w REAL, low_52w REAL, is_52w_high INTEGER, is_52w_low INTEGER,
		gap_pct REAL, true_range REAL, pct_off_52w_high REAL, pct_above_52w_low REAL,
		ret_1d REAL, ret_5d REAL, ret_1m REAL, ret_3m REAL, ret_6m REAL, ret_1y REAL,
		dollar_vol REAL, avg_dollar_vol20 REAL, rel_volume REAL, obv REAL, vwap_dist REAL, mfi14 REAL,
		golden_cross INTEGER, oversold_bounce INTEGER)`); err != nil {
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
		if _, err := stmt.Exec(
			r.Symbol, ts,
			nz(r.Close), nz(r.High), nz(r.Low), nz(r.Open),
			// Trend
			nz(r.SMA5), nz(r.SMA10), nz(r.SMA20), nz(r.SMA30), nz(r.SMA50), nz(r.SMA100), nz(r.SMA200),
			nz(r.EMA10), nz(r.EMA21), nz(r.EMA50), nz(r.EMA100), nz(r.EMA200),
			nz(r.PctFromSMA50), nz(r.PctFromSMA200), boolToInt(r.MAStack),
			// Momentum
			nz(r.RSI14), nz(r.MACD), nz(r.MACDSignal), nz(r.MACDHist),
			nz(r.StochK), nz(r.StochD), nz(r.WilliamsR), nz(r.CCI20),
			nz(r.ROC10), nz(r.ROC20), nz(r.ADX14), nz(r.DIPlus), nz(r.DIMinus),
			// Volatility
			nz(r.ATR14), nz(r.ATRPct),
			nz(r.BBUpper), nz(r.BBMiddle), nz(r.BBLower), nz(r.BBWidth), nz(r.BBPctB),
			nz(r.HistVol20),
			// Price structure
			nz(r.High52w), nz(r.Low52w), boolToInt(r.Is52wHigh), boolToInt(r.Is52wLow),
			nz(r.GapPct), nz(r.TrueRange), nz(r.PctOff52wHigh), nz(r.PctAbove52wLow),
			// Returns
			nz(r.Ret1d), nz(r.Ret5d), nz(r.Ret1m), nz(r.Ret3m), nz(r.Ret6m), nz(r.Ret1y),
			// Volume
			nz(r.DollarVol), nz(r.AvgDollarVol20), nz(r.RelVolume), nz(r.OBV), nz(r.VWAPDist), nz(r.MFI14),
			// Crosses
			boolToInt(r.IsGoldenCross), boolToInt(r.IsOversoldBounce),
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	// Update metadata
	d.snapshotDate = ts
	d.symbolCount = len(rows)
	d.snapshotID = fmt.Sprintf("%d", ts)

	return nil
}

// Metadata returns snapshot metadata for API responses.
func (d *DB) Metadata() Metadata {
	return Metadata{
		SnapshotID:  d.snapshotID,
		SymbolCount: d.symbolCount,
		SnapshotDate: d.snapshotDate,
	}
}

// Metadata holds snapshot metadata.
type Metadata struct {
	SnapshotID   string `json:"snapshot_id"`
	SymbolCount  int    `json:"symbol_count"`
	SnapshotDate int64  `json:"snapshot_date"`
}

// nz maps NaN → nil (SQL NULL).
func nz(v float64) any {
	if math.IsNaN(v) {
		return nil
	}
	return v
}

// boolToInt converts a boolean to an integer (0 or 1) for SQLite storage.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
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
