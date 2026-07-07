// Package store is a READ-ONLY reader of the cetus warehouse. It never writes.
//
// The schema is a contract owned by cetus-marketdata-pipeline — see its
// docs/DATA_DICTIONARY.md. We read the `adjusted_bars` view (split-adjusted
// OHLCV) and the `symbol_pipeline_state` universe. Opening read-only + WAL means
// we can read safely while an ingestion run is in progress.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"

	"cetus-marketdata-scanner/internal/model"

	_ "modernc.org/sqlite" // pure-Go driver, matching the pipeline
)

// Store wraps a read-only handle to the cetus warehouse.
type Store struct {
	db *sql.DB
}

// OpenReadOnly opens the warehouse read-only. WAL permits concurrent reads
// alongside the pipeline's single writer; we never write or run DDL.
func OpenReadOnly(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open cetus db %q read-only: %w", path, err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply pragma: %w", err)
	}
	// WAL permits many concurrent readers — size the pool for parallel scans so
	// worker goroutines don't serialise on a single connection.
	conns := runtime.NumCPU()
	if conns < 4 {
		conns = 4
	}
	db.SetMaxOpenConns(conns)
	db.SetMaxIdleConns(conns)
	return &Store{db: db}, nil
}

// Close releases the handle.
func (s *Store) Close() error { return s.db.Close() }

// Universe returns the symbols with successfully-ingested data (ascending), per
// symbol_pipeline_state. EMPTY/FAILED symbols are excluded — EMPTY has no data on
// the current feed, FAILED hit a real upstream error.
func (s *Store) Universe(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT symbol FROM symbol_pipeline_state WHERE status='SUCCESS' ORDER BY symbol`)
	if err != nil {
		return nil, fmt.Errorf("load universe: %w", err)
	}
	defer rows.Close()

	return collectSymbols(rows)
}

// UniverseExchange returns SUCCESS symbols listed on the given exchange
// (e.g. 'NASDAQ', 'NYSE'), ascending. This is listing venue — for a curated index
// like the S&P 500 use UniverseList against symbol_lists instead.
func (s *Store) UniverseExchange(ctx context.Context, exchange string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT p.symbol FROM symbol_pipeline_state p
JOIN symbols s ON s.symbol = p.symbol
WHERE p.status='SUCCESS' AND s.exchange = ?
ORDER BY p.symbol`, exchange)
	if err != nil {
		return nil, fmt.Errorf("load universe (exchange %q): %w", exchange, err)
	}
	defer rows.Close()
	return collectSymbols(rows)
}

// UniverseList returns SUCCESS symbols that belong to the named symbol_lists
// watchlist (e.g. 'sp500', 'nasdaq100'), ascending. Empty result = the list is not
// populated (the pipeline owns writing symbol_lists).
func (s *Store) UniverseList(ctx context.Context, listName string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT p.symbol FROM symbol_pipeline_state p
JOIN symbol_lists l ON l.symbol = p.symbol
WHERE p.status='SUCCESS' AND l.list_name = ?
ORDER BY p.symbol`, listName)
	if err != nil {
		return nil, fmt.Errorf("load universe (list %q): %w", listName, err)
	}
	defer rows.Close()
	return collectSymbols(rows)
}

// collectSymbols drains a single-column symbol result set.
func collectSymbols(rows *sql.Rows) ([]string, error) {
	var out []string
	for rows.Next() {
		var sym string
		if err := rows.Scan(&sym); err != nil {
			return nil, err
		}
		out = append(out, sym)
	}
	return out, rows.Err()
}

// OpsStats is warehouse / ingestion state for the admin dashboard.
type OpsStats struct {
	PipelineState   map[string]int // SUCCESS | PENDING | IN_FLIGHT | EMPTY | FAILED → count
	RegistrySymbols int            // rows in the symbols master registry
	EODBars         int64          // nominal bars ingested
	Splits          int            // corporate actions in the split ledger
}

// Total is the tracked symbol count across all pipeline states.
func (s OpsStats) Total() int {
	t := 0
	for _, n := range s.PipelineState {
		t += n
	}
	return t
}

// Count returns the symbols in a given pipeline state.
func (s OpsStats) Count(status string) int { return s.PipelineState[status] }

// Pct returns a pipeline state's share of the tracked universe (0 if none).
func (s OpsStats) Pct(status string) float64 {
	if t := s.Total(); t > 0 {
		return 100 * float64(s.PipelineState[status]) / float64(t)
	}
	return 0
}

// Stats gathers operational counts for the dashboard (cheap aggregate queries).
func (s *Store) Stats(ctx context.Context) (OpsStats, error) {
	out := OpsStats{PipelineState: map[string]int{}}
	rows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM symbol_pipeline_state GROUP BY status`)
	if err != nil {
		return out, fmt.Errorf("stats pipeline_state: %w", err)
	}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			rows.Close()
			return out, err
		}
		out.PipelineState[st] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	for _, q := range []struct {
		sql string
		dst any
	}{
		{`SELECT COUNT(*) FROM symbols`, &out.RegistrySymbols},
		{`SELECT COUNT(*) FROM eod_bars`, &out.EODBars},
		{`SELECT COUNT(*) FROM split_factors`, &out.Splits},
	} {
		if err := s.db.QueryRowContext(ctx, q.sql).Scan(q.dst); err != nil {
			return out, fmt.Errorf("stats %q: %w", q.sql, err)
		}
	}
	return out, nil
}

// LoadAdjustedBars returns split-adjusted daily bars for symbol with
// timestamp >= since, ascending, straight from the adjusted_bars view (no
// client-side split math).
func (s *Store) LoadAdjustedBars(ctx context.Context, symbol string, since int64) ([]model.Bar, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT symbol, timestamp, open, high, low, close, volume, COALESCE(vwap, 0), source
FROM adjusted_bars
WHERE symbol = ? AND timestamp >= ?
ORDER BY timestamp ASC`, symbol, since)
	if err != nil {
		return nil, fmt.Errorf("load adjusted bars %s: %w", symbol, err)
	}
	defer rows.Close()

	var out []model.Bar
	for rows.Next() {
		var b model.Bar
		if err := rows.Scan(&b.Symbol, &b.Timestamp, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume, &b.VWAP, &b.Source); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
