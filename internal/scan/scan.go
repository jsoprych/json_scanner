// Package scan orchestrates a concurrent, whole-universe snapshot: it fans symbols
// across a worker pool, loads each one's bars, and derives its SnapshotRow. It is
// decoupled from the warehouse via the BarLoader interface, so it's testable with a
// fake and reusable by any caller (CLI digest, future API server).
package scan

import (
	"context"
	"log/slog"
	"math"
	"runtime"
	"sort"
	"sync"
	"time"

	"cetus-marketdata-scanner/internal/model"
	"cetus-marketdata-scanner/internal/screen"
)

// BarLoader supplies split-adjusted bars for a symbol since a Unix-seconds bound.
// *store.Store satisfies it; tests use a fake.
type BarLoader interface {
	LoadAdjustedBars(ctx context.Context, symbol string, since int64) ([]model.Bar, error)
}

// Options tunes a universe scan.
type Options struct {
	Since        int64   // load bars with timestamp >= Since (Unix seconds)
	MinDollarVol float64 // liquidity floor; rows below are dropped
	Workers      int     // parallelism; <=0 → runtime.NumCPU()
}

// Result is the cross-sectional snapshot plus scan stats.
type Result struct {
	Rows    []screen.SnapshotRow // eligible rows, sorted by symbol (deterministic)
	Scanned int                  // symbols that returned data
	Day     time.Time            // latest bar date seen (UTC)
}

// Universe scans all symbols concurrently and returns the eligible snapshot. It
// respects ctx cancellation. Load errors are logged and skipped, never fatal — one
// bad symbol shouldn't sink the whole digest.
func Universe(ctx context.Context, loader BarLoader, symbols []string, opts Options, log *slog.Logger) Result {
	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > len(symbols) && len(symbols) > 0 {
		workers = len(symbols)
	}

	jobs := make(chan string)
	type item struct {
		row screen.SnapshotRow
		ts  int64
		ok  bool // passed the liquidity/warm-up filter
	}
	results := make(chan item, workers)

	// Feeder: enqueue symbols, stop early on cancellation.
	go func() {
		defer close(jobs)
		for _, s := range symbols {
			select {
			case <-ctx.Done():
				return
			case jobs <- s:
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for sym := range jobs {
				if ctx.Err() != nil {
					return
				}
				bars, err := loader.LoadAdjustedBars(ctx, sym, opts.Since)
				if err != nil {
					log.Error("load bars failed", "symbol", sym, "error", err)
					continue
				}
				if len(bars) == 0 {
					continue
				}
				row, ok := screen.Build(sym, bars)
				it := item{ts: bars[len(bars)-1].Timestamp}
				if ok && !math.IsNaN(row.DollarVol) && row.DollarVol >= opts.MinDollarVol {
					it.row, it.ok = row, true
				}
				results <- it
			}
		}()
	}
	go func() { wg.Wait(); close(results) }()

	var res Result
	var maxTs int64
	for it := range results {
		res.Scanned++
		if it.ts > maxTs {
			maxTs = it.ts
		}
		if it.ok {
			res.Rows = append(res.Rows, it.row)
		}
	}

	// Deterministic order regardless of worker scheduling (stabilises rank ties).
	sort.Slice(res.Rows, func(i, j int) bool { return res.Rows[i].Symbol < res.Rows[j].Symbol })

	res.Day = time.Now().UTC()
	if maxTs > 0 {
		res.Day = time.Unix(maxTs, 0).UTC()
	}
	return res
}

// SnapshotStore is the interface for storing historical snapshots.
type SnapshotStore interface {
	LoadHistory(rows []screen.SnapshotRow, barTs, snapshotDate int64) error
	Cleanup(keepDays int) (int, error)
}

// BackfillOptions tunes a historical snapshot backfill.
type BackfillOptions struct {
	Days         int     // number of historical days to backfill
	KeepDays     int     // retention period for cleanup (0 = no cleanup)
	MinDollarVol float64 // liquidity floor
	Workers      int     // parallelism
}

// BackfillSnapshots builds historical snapshots for the past N days.
// For each day, it loads bars up to that day, computes indicators, and stores the snapshot.
// This enables backtesting and audit trails.
func BackfillSnapshots(ctx context.Context, loader BarLoader, store SnapshotStore, symbols []string, opts BackfillOptions, log *slog.Logger) (int, error) {
	if opts.Days <= 0 {
		return 0, nil
	}
	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	now := time.Now().UTC()
	var backfilled int

	for dayOffset := opts.Days - 1; dayOffset >= 0; dayOffset-- {
		if ctx.Err() != nil {
			return backfilled, ctx.Err()
		}
		snapshotDay := now.AddDate(0, 0, -dayOffset)
		snapshotDate := time.Date(snapshotDay.Year(), snapshotDay.Month(), snapshotDay.Day(), 0, 0, 0, 0, time.UTC).Unix()

		// Load bars up to this day (400 days lookback covers 52-week high)
		since := snapshotDay.AddDate(0, 0, -400).Unix()
		res := Universe(ctx, loader, symbols, Options{
			Since:        since,
			MinDollarVol: opts.MinDollarVol,
			Workers:      workers,
		}, log)

		if len(res.Rows) == 0 {
			log.Warn("no data for snapshot", "date", snapshotDay.Format("2006-01-02"))
			continue
		}

		if err := store.LoadHistory(res.Rows, res.Day.Unix(), snapshotDate); err != nil {
			log.Error("store snapshot failed", "date", snapshotDay.Format("2006-01-02"), "error", err)
			continue
		}
		backfilled++
		log.Info("snapshot stored", "date", snapshotDay.Format("2006-01-02"), "symbols", len(res.Rows))
	}

	// Cleanup old snapshots
	if opts.KeepDays > 0 {
		deleted, err := store.Cleanup(opts.KeepDays)
		if err != nil {
			log.Error("cleanup failed", "error", err)
		} else if deleted > 0 {
			log.Info("snapshots cleaned up", "deleted_dates", deleted, "keep_days", opts.KeepDays)
		}
	}

	return backfilled, nil
}
