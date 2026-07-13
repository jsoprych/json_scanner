// Package snapshot materializes the cross-sectional feature snapshot into a SQLite
// table so studies can be plain SQL WHERE/ORDER BY over it — SQL is the study
// language, no DSL. The table is tiny (one row per symbol), so an in-memory DB is
// instant; an on-disk path lets you also query it by hand with the sqlite3 CLI.
package snapshot

import (
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"cetus-marketdata-scanner/internal/dblog"
	"cetus-marketdata-scanner/internal/screen"
	"cetus-marketdata-scanner/internal/study"

	_ "modernc.org/sqlite"
)

// DB is a snapshot store.
type DB struct {
	db           *dblog.DB
	snapshotID   string
	symbolCount  int
	snapshotDate int64
	activeDate   int64
}

// Open opens a snapshot DB with logging.
func Open(path string, log *slog.Logger) (*DB, error) {
	dsn := ":memory:"
	if path != "" && path != ":memory:" {
		dsn = "file:" + path
	}
	rawDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	rawDB.SetMaxOpenConns(1)
	rawDB.Exec("PRAGMA foreign_keys = ON")

	db := dblog.New(rawDB, log)
	d := &DB{db: db}
	if err := d.ensureTable(); err != nil {
		rawDB.Close()
		return nil, fmt.Errorf("snapshot schema: %w", err)
	}
	return d, nil
}

// OpenTest opens a snapshot DB for test use (no external logger).
func OpenTest(path string) (*DB, error) {
	return Open(path, slog.Default())
}

// Close releases the DB.
func (d *DB) Close() error { return d.db.Close() }

// RawDB returns the underlying *sql.DB for migrations and raw access.
func (d *DB) RawDB() *sql.DB { return d.db.DB() }

// LogDB returns the loggable DB wrapper for stores that need logging.
func (d *DB) LogDB() *dblog.DB { return d.db }

// SnapshotBatch represents a single snapshot to be batched.
type SnapshotBatch struct {
	Rows         []screen.SnapshotRow
	BarTs        int64
	SnapshotDate int64
}

// createTableSQL is the single source of truth for the snapshot table schema.
// All Load/LoadHistory methods use this constant via ensureTable().
const createTableSQL = `CREATE TABLE IF NOT EXISTS snapshot(
	snapshot_date INTEGER, symbol TEXT, timestamp INTEGER,
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
	golden_cross INTEGER, oversold_bounce INTEGER,
	psar REAL, aroon_up REAL, aroon_down REAL, aroon_osc REAL,
	keltner_upper REAL, keltner_mid REAL, keltner_lower REAL,
	cmf20 REAL, ultimate_osc REAL,
	PRIMARY KEY (snapshot_date, symbol))`

// ensureTable creates the snapshot table if it doesn't exist. If the table
// exists but has a stale column count, it drops and recreates to match the
// current schema — no migration system needed.
func (d *DB) ensureTable() error {
	var count int
	if err := d.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('snapshot')").Scan(&count); err != nil {
		count = 0
	}
	if count > 0 && count != len(columns) {
		d.db.Exec("DROP TABLE snapshot")
	}
	_, err := d.db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("create snapshot table: %w", err)
	}
	return nil
}

// columns of the snapshot table, in insert order.
var columns = []string{
	"snapshot_date", "symbol", "timestamp", "close", "high", "low", "open",
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
	// Additional indicators
	"psar", "aroon_up", "aroon_down", "aroon_osc",
	"keltner_upper", "keltner_mid", "keltner_lower",
	"cmf20", "ultimate_osc",
}

// Load clears the snapshot table for this timestamp and inserts fresh rows.
// NaN → NULL so SQL comparisons behave: an under-warmed feature is NULL.
// Uses DELETE (not DROP) so historical data from backfill is preserved.
func (d *DB) Load(rows []screen.SnapshotRow, ts int64) error {
	if err := d.ensureTable(); err != nil {
		return err
	}
	if _, err := d.db.Exec("DELETE FROM snapshot WHERE snapshot_date = ?", ts); err != nil {
		return fmt.Errorf("clear snapshot date: %w", err)
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ph := strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",")
	stmt, err := tx.Prepare("INSERT INTO snapshot(" + strings.Join(columns, ",") + ") VALUES(" + ph + ")")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(
			ts, r.Symbol, ts,
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
				nz(r.PSAR), nz(r.AroonUp), nz(r.AroonDown), nz(r.AroonOsc),
				nz(r.KeltnerUpper), nz(r.KeltnerMid), nz(r.KeltnerLower),
				nz(r.CMF20), nz(r.UltimateOsc),
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
	d.activeDate = ts
	d.symbolCount = len(rows)
	d.snapshotID = fmt.Sprintf("%d", ts)

	return nil
}

// LoadHistoryBatch inserts multiple snapshots in a single transaction.
// This is much more efficient than calling LoadHistoryInsert() for each date.
func (d *DB) LoadHistoryBatch(snapshots []SnapshotBatch) error {
	if len(snapshots) == 0 {
		return nil
	}

	if err := d.ensureTable(); err != nil {
		return err
	}

	// Single transaction for all snapshots
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ph := strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",")
	stmt, err := tx.Prepare("INSERT OR IGNORE INTO snapshot(" + strings.Join(columns, ",") + ") VALUES(" + ph + ")")
	if err != nil {
		return err
	}
	defer stmt.Close()

	totalRows := 0
	for _, snap := range snapshots {
		for _, r := range snap.Rows {
			if _, err := stmt.Exec(
				snap.SnapshotDate, r.Symbol, snap.BarTs,
				nz(r.Close), nz(r.High), nz(r.Low), nz(r.Open),
				nz(r.SMA5), nz(r.SMA10), nz(r.SMA20), nz(r.SMA30), nz(r.SMA50), nz(r.SMA100), nz(r.SMA200),
				nz(r.EMA10), nz(r.EMA21), nz(r.EMA50), nz(r.EMA100), nz(r.EMA200),
				nz(r.PctFromSMA50), nz(r.PctFromSMA200), boolToInt(r.MAStack),
				nz(r.RSI14), nz(r.MACD), nz(r.MACDSignal), nz(r.MACDHist),
				nz(r.StochK), nz(r.StochD), nz(r.WilliamsR), nz(r.CCI20),
				nz(r.ROC10), nz(r.ROC20), nz(r.ADX14), nz(r.DIPlus), nz(r.DIMinus),
				nz(r.ATR14), nz(r.ATRPct),
				nz(r.BBUpper), nz(r.BBMiddle), nz(r.BBLower), nz(r.BBWidth), nz(r.BBPctB),
				nz(r.HistVol20),
				nz(r.High52w), nz(r.Low52w), boolToInt(r.Is52wHigh), boolToInt(r.Is52wLow),
				nz(r.GapPct), nz(r.TrueRange), nz(r.PctOff52wHigh), nz(r.PctAbove52wLow),
				nz(r.Ret1d), nz(r.Ret5d), nz(r.Ret1m), nz(r.Ret3m), nz(r.Ret6m), nz(r.Ret1y),
				nz(r.DollarVol), nz(r.AvgDollarVol20), nz(r.RelVolume), nz(r.OBV), nz(r.VWAPDist), nz(r.MFI14),
				boolToInt(r.IsGoldenCross), boolToInt(r.IsOversoldBounce),
				nz(r.PSAR), nz(r.AroonUp), nz(r.AroonDown), nz(r.AroonOsc),
				nz(r.KeltnerUpper), nz(r.KeltnerMid), nz(r.KeltnerLower),
				nz(r.CMF20), nz(r.UltimateOsc),
			); err != nil {
				return err
			}
			totalRows++
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Update metadata to reflect the latest snapshot
	if len(snapshots) > 0 {
		last := snapshots[len(snapshots)-1]
		d.snapshotDate = last.SnapshotDate
		d.activeDate = last.SnapshotDate
		d.symbolCount = len(last.Rows)
		d.snapshotID = fmt.Sprintf("%d", last.SnapshotDate)
	}

	return nil
}

// LoadHistoryInsert inserts a snapshot for a specific date without deleting existing data.
// Uses INSERT OR IGNORE to skip symbols that already exist for this date.
// This is non-destructive and safe for incremental backfills.
func (d *DB) LoadHistoryInsert(rows []screen.SnapshotRow, barTs, snapshotDate int64) error {
	if err := d.ensureTable(); err != nil {
		return err
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",")
	stmt, err := tx.Prepare("INSERT OR IGNORE INTO snapshot(" + strings.Join(columns, ",") + ") VALUES(" + ph + ")")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(
			snapshotDate, r.Symbol, barTs,
			nz(r.Close), nz(r.High), nz(r.Low), nz(r.Open),
			nz(r.SMA5), nz(r.SMA10), nz(r.SMA20), nz(r.SMA30), nz(r.SMA50), nz(r.SMA100), nz(r.SMA200),
			nz(r.EMA10), nz(r.EMA21), nz(r.EMA50), nz(r.EMA100), nz(r.EMA200),
			nz(r.PctFromSMA50), nz(r.PctFromSMA200), boolToInt(r.MAStack),
			nz(r.RSI14), nz(r.MACD), nz(r.MACDSignal), nz(r.MACDHist),
			nz(r.StochK), nz(r.StochD), nz(r.WilliamsR), nz(r.CCI20),
			nz(r.ROC10), nz(r.ROC20), nz(r.ADX14), nz(r.DIPlus), nz(r.DIMinus),
			nz(r.ATR14), nz(r.ATRPct),
			nz(r.BBUpper), nz(r.BBMiddle), nz(r.BBLower), nz(r.BBWidth), nz(r.BBPctB),
			nz(r.HistVol20),
			nz(r.High52w), nz(r.Low52w), boolToInt(r.Is52wHigh), boolToInt(r.Is52wLow),
			nz(r.GapPct), nz(r.TrueRange), nz(r.PctOff52wHigh), nz(r.PctAbove52wLow),
			nz(r.Ret1d), nz(r.Ret5d), nz(r.Ret1m), nz(r.Ret3m), nz(r.Ret6m), nz(r.Ret1y),
			nz(r.DollarVol), nz(r.AvgDollarVol20), nz(r.RelVolume), nz(r.OBV), nz(r.VWAPDist), nz(r.MFI14),
			boolToInt(r.IsGoldenCross), boolToInt(r.IsOversoldBounce),
				nz(r.PSAR), nz(r.AroonUp), nz(r.AroonDown), nz(r.AroonOsc),
				nz(r.KeltnerUpper), nz(r.KeltnerMid), nz(r.KeltnerLower),
				nz(r.CMF20), nz(r.UltimateOsc),
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	// Update metadata to reflect the latest snapshot
	d.snapshotDate = snapshotDate
	d.activeDate = snapshotDate
	d.symbolCount = len(rows)
	d.snapshotID = fmt.Sprintf("%d", snapshotDate)

	return nil
}

// LoadHistory inserts a snapshot for a specific date without dropping existing data.
// The snapshot_date is the date the snapshot was computed for (typically the latest bar date).
// Multiple snapshots can coexist, keyed by (snapshot_date, symbol).
func (d *DB) LoadHistory(rows []screen.SnapshotRow, barTs, snapshotDate int64) error {
	if err := d.ensureTable(); err != nil {
		return err
	}

	// Delete existing snapshot for this date (replace, not append)
	if _, err := d.db.Exec("DELETE FROM snapshot WHERE snapshot_date = ?", snapshotDate); err != nil {
		return fmt.Errorf("delete old snapshot: %w", err)
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ph := strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",")
	stmt, err := tx.Prepare("INSERT INTO snapshot(" + strings.Join(columns, ",") + ") VALUES(" + ph + ")")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(
			snapshotDate, r.Symbol, barTs,
			nz(r.Close), nz(r.High), nz(r.Low), nz(r.Open),
			nz(r.SMA5), nz(r.SMA10), nz(r.SMA20), nz(r.SMA30), nz(r.SMA50), nz(r.SMA100), nz(r.SMA200),
			nz(r.EMA10), nz(r.EMA21), nz(r.EMA50), nz(r.EMA100), nz(r.EMA200),
			nz(r.PctFromSMA50), nz(r.PctFromSMA200), boolToInt(r.MAStack),
			nz(r.RSI14), nz(r.MACD), nz(r.MACDSignal), nz(r.MACDHist),
			nz(r.StochK), nz(r.StochD), nz(r.WilliamsR), nz(r.CCI20),
			nz(r.ROC10), nz(r.ROC20), nz(r.ADX14), nz(r.DIPlus), nz(r.DIMinus),
			nz(r.ATR14), nz(r.ATRPct),
			nz(r.BBUpper), nz(r.BBMiddle), nz(r.BBLower), nz(r.BBWidth), nz(r.BBPctB),
			nz(r.HistVol20),
			nz(r.High52w), nz(r.Low52w), boolToInt(r.Is52wHigh), boolToInt(r.Is52wLow),
			nz(r.GapPct), nz(r.TrueRange), nz(r.PctOff52wHigh), nz(r.PctAbove52wLow),
			nz(r.Ret1d), nz(r.Ret5d), nz(r.Ret1m), nz(r.Ret3m), nz(r.Ret6m), nz(r.Ret1y),
			nz(r.DollarVol), nz(r.AvgDollarVol20), nz(r.RelVolume), nz(r.OBV), nz(r.VWAPDist), nz(r.MFI14),
			boolToInt(r.IsGoldenCross), boolToInt(r.IsOversoldBounce),
				nz(r.PSAR), nz(r.AroonUp), nz(r.AroonDown), nz(r.AroonOsc),
				nz(r.KeltnerUpper), nz(r.KeltnerMid), nz(r.KeltnerLower),
				nz(r.CMF20), nz(r.UltimateOsc),
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	// Update metadata to reflect the latest snapshot
	d.snapshotDate = snapshotDate
	d.activeDate = snapshotDate
	d.symbolCount = len(rows)
	d.snapshotID = fmt.Sprintf("%d", snapshotDate)

	return nil
}

// HasSnapshot checks if a snapshot exists for the given date.
func (d *DB) HasSnapshot(snapshotDate int64) (bool, error) {
	var n int
	if err := d.db.QueryRow("SELECT COUNT(*) FROM snapshot WHERE snapshot_date = ?", snapshotDate).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// ListSnapshots returns the available snapshot dates (Unix seconds), newest first.
func (d *DB) ListSnapshots() ([]int64, error) {
	rows, err := d.db.Query("SELECT DISTINCT snapshot_date FROM snapshot ORDER BY snapshot_date DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dates []int64
	for rows.Next() {
		var ts int64
		if err := rows.Scan(&ts); err != nil {
			return nil, err
		}
		dates = append(dates, ts)
	}
	return dates, rows.Err()
}

// SetActive sets the active snapshot date for Run queries. Returns an error if the
// date doesn't exist in the snapshot table.
func (d *DB) SetActive(snapshotDate int64) error {
	var n int
	if err := d.db.QueryRow("SELECT COUNT(*) FROM snapshot WHERE snapshot_date = ?", snapshotDate).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("snapshot date %d not found", snapshotDate)
	}
	d.activeDate = snapshotDate
	return nil
}

// Cleanup deletes snapshots older than keepDays. Returns the number of dates deleted.
func (d *DB) Cleanup(keepDays int) (int, error) {
	if keepDays <= 0 {
		return 0, nil
	}
	// Count distinct dates before cleanup
	var before int
	if err := d.db.QueryRow("SELECT COUNT(DISTINCT snapshot_date) FROM snapshot").Scan(&before); err != nil {
		return 0, err
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -keepDays).Unix()
	if _, err := d.db.Exec("DELETE FROM snapshot WHERE snapshot_date < ?", cutoff); err != nil {
		return 0, err
	}
	var after int
	if err := d.db.QueryRow("SELECT COUNT(DISTINCT snapshot_date) FROM snapshot").Scan(&after); err != nil {
		return 0, err
	}
	return before - after, nil
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

// SymbolClose returns the close price for one symbol on one snapshot date.
// Uses the PK index (snapshot_date, symbol) — O(log N), not a full scan.
func (d *DB) SymbolClose(symbol string, date int64) (float64, error) {
	var close sql.NullFloat64
	err := d.db.QueryRow(
		"SELECT close FROM snapshot WHERE snapshot_date = ? AND symbol = ?",
		date, symbol,
	).Scan(&close)
	if err != nil {
		return 0, err
	}
	if !close.Valid {
		return 0, sql.ErrNoRows
	}
	return close.Float64, nil
}

// NearestDate returns the earliest snapshot_date >= from.
// Returns sql.ErrNoRows if no snapshot exists at or after from.
func (d *DB) NearestDate(from int64) (int64, error) {
	var date int64
	err := d.db.QueryRow(
		"SELECT MIN(snapshot_date) FROM snapshot WHERE snapshot_date >= ?", from,
	).Scan(&date)
	if err != nil {
		return 0, err
	}
	return date, nil
}

// Run executes a study's WHERE / ORDER BY / LIMIT and returns matches.
//
// The study SQL is TRUSTED — it comes from the Global user's local config. When
// untrusted, user-authored SQL arrives (multi-user, paid tier), it must be
// validated/sandboxed (allow-list of columns/operators) before reaching here.
// Queries are scoped to the active snapshot date (set by SetActive or Load).
func (d *DB) Run(s study.Study) ([]Match, error) {
	where := strings.TrimSpace(s.Where)
	if where == "" {
		where = "1=1"
	}

	// Validate SQL against real schema — catches injection, unknown columns,
	// and syntax errors for free (LIMIT 0 reads zero rows).
	test := "SELECT 1 FROM snapshot WHERE snapshot_date = ? AND (" + where + ")"
	if s.OrderBy != "" {
		test += " ORDER BY " + s.OrderBy
	}
	if s.Limit > 0 {
		test += fmt.Sprintf(" LIMIT %d", s.Limit)
	} else {
		test += " LIMIT 0"
	}
	if _, err := d.db.Exec("EXPLAIN "+test, d.activeDate); err != nil {
		return nil, fmt.Errorf("study %q: invalid SQL: %w", s.Key, err)
	}

	q := "SELECT symbol, close, rsi14, ret_3m, dollar_vol FROM snapshot WHERE snapshot_date = ? AND (" + where + ")"
	if s.OrderBy != "" {
		q += " ORDER BY " + s.OrderBy
	}
	if s.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", s.Limit)
	}
	rows, err := d.db.Query(q, d.activeDate)
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
