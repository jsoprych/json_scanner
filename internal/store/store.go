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
	db   *sql.DB
	bars string // the bars read surface in use (published_bars|clean_bars|adjusted_bars)
}

// barsPreference is the read surface in order of preference: the materialized,
// quarantine-free published_bars (the producer's hot path); else the live clean_bars
// view (quarantine-free, but recomputed); else adjusted_bars (keeps quarantined bars).
// Per docs/DOWNSTREAM.md — prefer published_bars, never filter/adjust client-side.
var barsPreference = []string{"published_bars", "clean_bars", "adjusted_bars"}

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

	s := &Store{db: db, bars: "adjusted_bars"}
	for _, t := range barsPreference {
		var n int
		if err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE name=? AND type IN ('table','view')", t).Scan(&n); err == nil && n > 0 {
			s.bars = t
			break
		}
	}
	return s, nil
}

// Close releases the handle.
func (s *Store) Close() error { return s.db.Close() }

// BarsTable reports which read surface the store resolved to.
func (s *Store) BarsTable() string { return s.bars }

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

// UniverseExchange returns SUCCESS common-stock symbols on the given exchange
// (e.g. 'NASDAQ', 'NYSE') — listing venue, not an index.
func (s *Store) UniverseExchange(ctx context.Context, exchange string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT p.symbol FROM symbol_pipeline_state p
JOIN symbols s ON s.symbol = p.symbol
WHERE p.status='SUCCESS' AND s.exchange = ? AND s.security_type='common'
ORDER BY p.symbol`, exchange)
	if err != nil {
		return nil, fmt.Errorf("load universe (exchange %q): %w", exchange, err)
	}
	defer rows.Close()
	return collectSymbols(rows)
}

// UniverseIndex returns SUCCESS common-stock CURRENT members of an index
// (e.g. 'r3000', 'sp500') per index_membership. Empty result = the index isn't
// seeded yet (the pipeline owns index_membership) — callers may fall back.
func (s *Store) UniverseIndex(ctx context.Context, code string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT p.symbol FROM symbol_pipeline_state p
JOIN index_membership m ON m.symbol = p.symbol AND m.index_code = ? AND m.removed IS NULL
JOIN symbols s ON s.symbol = p.symbol AND s.security_type='common'
WHERE p.status='SUCCESS' ORDER BY p.symbol`, code)
	if err != nil {
		return nil, fmt.Errorf("load universe (index %q): %w", code, err)
	}
	defer rows.Close()
	return collectSymbols(rows)
}

// UniverseCommon returns all SUCCESS common-stock symbols (drops warrants/units/
// rights/ETFs via security_type). The safe fallback when no index is seeded.
func (s *Store) UniverseCommon(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT p.symbol FROM symbol_pipeline_state p
JOIN symbols s ON s.symbol = p.symbol
WHERE p.status='SUCCESS' AND s.security_type='common'
ORDER BY p.symbol`)
	if err != nil {
		return nil, fmt.Errorf("load universe (common): %w", err)
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

// LoadAdjustedBars returns best-corrected, split-adjusted, quality-gated daily bars
// for symbol with timestamp >= since, ascending, from the resolved read surface
// (published_bars → clean_bars → adjusted_bars). No client-side split or quality math.
func (s *Store) LoadAdjustedBars(ctx context.Context, symbol string, since int64) ([]model.Bar, error) {
	// s.bars is chosen from a fixed allow-list (never user input) — safe to inline.
	rows, err := s.db.QueryContext(ctx, `
SELECT symbol, timestamp, open, high, low, close, volume, COALESCE(vwap, 0), source
FROM `+s.bars+`
WHERE symbol = ? AND timestamp >= ?
ORDER BY timestamp ASC`, symbol, since)
	if err != nil {
		return nil, fmt.Errorf("load bars %s from %s: %w", symbol, s.bars, err)
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
