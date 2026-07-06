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
