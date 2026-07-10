package scan

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"cetus-marketdata-scanner/internal/model"
	_ "modernc.org/sqlite"
)

// BarCache provides bulk-loaded bar data using SQLite in-memory tables.
// We leverage SQLite's battle-tested storage, indexing, and query optimization
// instead of reinventing with custom Go maps and binary search.
type BarCache struct {
	db     *sql.DB
	loader BarLoader
	log    *slog.Logger
}

// NewBarCache creates a new SQLite-backed bar cache.
func NewBarCache(loader BarLoader, log *slog.Logger) (*BarCache, error) {
	// Use in-memory SQLite for fast access
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("open cache db: %w", err)
	}

	// Use a single connection for in-memory database to ensure table persistence
	db.SetMaxOpenConns(1)

	// Create bars table with proper indexing
	_, err = db.Exec(`
		CREATE TABLE bars (
			symbol TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			open REAL,
			high REAL,
			low REAL,
			close REAL,
			volume INTEGER,
			PRIMARY KEY (symbol, timestamp)
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create bars table: %w", err)
	}

	// Create index for fast symbol lookups
	_, err = db.Exec(`CREATE INDEX idx_bars_symbol ON bars(symbol)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create index: %w", err)
	}

	return &BarCache{
		db:     db,
		loader: loader,
		log:    log,
	}, nil
}

// BulkLoad loads bars for all symbols from startDate - lookbackDays to endDate.
// Uses SQLite transactions for efficient bulk inserts.
func (c *BarCache) BulkLoad(ctx context.Context, symbols []string, startDate, endDate time.Time, lookbackDays int) error {
	start := startDate.AddDate(0, 0, -lookbackDays)
	since := start.Unix()

	c.log.Info("bulk loading bars into SQLite cache", "symbols", len(symbols), "start", start.Format("2006-01-02"), "end", endDate.Format("2006-01-02"), "lookback_days", lookbackDays)

	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO bars (symbol, timestamp, open, high, low, close, volume) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	loaded := 0
	for _, sym := range symbols {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		bars, err := c.loader.LoadAdjustedBars(ctx, sym, since)
		if err != nil {
			c.log.Warn("failed to load bars", "symbol", sym, "error", err)
			continue
		}

		for _, bar := range bars {
			_, err := stmt.Exec(bar.Symbol, bar.Timestamp, bar.Open, bar.High, bar.Low, bar.Close, bar.Volume)
			if err != nil {
				c.log.Warn("failed to insert bar", "symbol", sym, "timestamp", bar.Timestamp, "error", err)
				continue
			}
		}

		loaded++
		if loaded%1000 == 0 {
			c.log.Info("bulk load progress", "loaded", loaded, "total", len(symbols))
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	c.log.Info("bulk load complete", "symbols_loaded", loaded, "total_symbols", len(symbols))
	return nil
}

// GetBarsForSymbol returns bars for ONE symbol up to the given date (inclusive).
// Uses SQL WHERE clause - let SQLite handle the filtering.
func (c *BarCache) GetBarsForSymbol(symbol string, upTo time.Time) ([]model.Bar, error) {
	upToUnix := upTo.Unix()

	rows, err := c.db.Query(`
		SELECT symbol, timestamp, open, high, low, close, volume
		FROM bars
		WHERE symbol = ? AND timestamp <= ?
		ORDER BY timestamp ASC
	`, symbol, upToUnix)
	if err != nil {
		return nil, fmt.Errorf("query bars: %w", err)
	}
	defer rows.Close()

	var bars []model.Bar
	for rows.Next() {
		var bar model.Bar
		err := rows.Scan(&bar.Symbol, &bar.Timestamp, &bar.Open, &bar.High, &bar.Low, &bar.Close, &bar.Volume)
		if err != nil {
			return nil, fmt.Errorf("scan bar: %w", err)
		}
		bars = append(bars, bar)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return bars, nil
}

// GetQualifyingSymbols returns symbols that meet the dollar_vol threshold.
// Uses SQL aggregation to filter - let SQLite do the heavy lifting.
// Returns symbols sorted alphabetically.
func (c *BarCache) GetQualifyingSymbols(upTo time.Time, minDollarVol float64) ([]string, error) {
	upToUnix := upTo.Unix()

	// Use SQL to filter symbols by dollar_vol threshold
	// dollar_vol = close * volume (using the latest bar for each symbol)
	rows, err := c.db.Query(`
		SELECT symbol
		FROM bars
		WHERE timestamp <= ?
		GROUP BY symbol
		HAVING MAX(close * volume) >= ?
		ORDER BY symbol ASC
	`, upToUnix, minDollarVol)
	if err != nil {
		return nil, fmt.Errorf("query qualifying symbols: %w", err)
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, fmt.Errorf("scan symbol: %w", err)
		}
		symbols = append(symbols, symbol)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return symbols, nil
}

// Close closes the SQLite database connection.
func (c *BarCache) Close() error {
	return c.db.Close()
}
