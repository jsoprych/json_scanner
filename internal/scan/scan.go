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
	"cetus-marketdata-scanner/internal/snapshot"
)

// BarLoader supplies split-adjusted bars for a symbol since a Unix-seconds bound.
// *store.Store satisfies it; tests use a fake.
type BarLoader interface {
	LoadAdjustedBars(ctx context.Context, symbol string, since int64) ([]model.Bar, error)
}

// DateLoader returns the latest bar date in the warehouse.
type DateLoader interface {
	LatestBarDate(ctx context.Context) (time.Time, error)
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

// UniverseFromCache generates a snapshot from cached bars for a specific date.
// Uses SQLite for filtering and sorting - let the database do the work.
func UniverseFromCache(ctx context.Context, cache *BarCache, symbols []string, snapshotDay time.Time, minDollarVol float64, workers int, log *slog.Logger) Result {
	// Step 1: Use SQL to get symbols that meet dollar_vol threshold
	qualifyingSymbols, err := cache.GetQualifyingSymbols(snapshotDay, minDollarVol)
	if err != nil {
		log.Error("failed to get qualifying symbols from cache", "error", err)
		return Result{}
	}

	// Step 2: For each qualifying symbol, get bars and compute indicators
	var rows []screen.SnapshotRow
	var maxTs int64
	scanned := 0

	for _, sym := range qualifyingSymbols {
		scanned++
		
		// Get bars for this symbol up to snapshot day
		bars, err := cache.GetBarsForSymbol(sym, snapshotDay)
		if err != nil {
			log.Warn("failed to get bars from cache", "symbol", sym, "error", err)
			continue
		}
		
		if len(bars) == 0 {
			continue
		}
		
		// Track max timestamp
		if bars[len(bars)-1].Timestamp > maxTs {
			maxTs = bars[len(bars)-1].Timestamp
		}
		
		// Compute indicators (this is where Go shines - complex math)
		row, ok := screen.Build(sym, bars)
		if ok {
			rows = append(rows, row)
		}
	}

	// Results are already sorted by SQLite (ORDER BY symbol in GetQualifyingSymbols)
	day := time.Now().UTC()
	if maxTs > 0 {
		day = time.Unix(maxTs, 0).UTC()
	}
	
	return Result{
		Rows:    rows,
		Scanned: scanned,
		Day:     day,
	}
}

// SnapshotStore is the interface for storing historical snapshots.
type SnapshotStore interface {
	LoadHistory(rows []screen.SnapshotRow, barTs, snapshotDate int64) error
	LoadHistoryInsert(rows []screen.SnapshotRow, barTs, snapshotDate int64) error
	LoadHistoryBatch(snapshots []snapshot.SnapshotBatch) error
	HasSnapshot(snapshotDate int64) (bool, error)
	Cleanup(keepDays int) (int, error)
}

// BackfillOptions tunes a historical snapshot backfill.
type BackfillOptions struct {
	Days         int     // number of historical days to backfill
	KeepDays     int     // retention period for cleanup (0 = no cleanup)
	MinDollarVol float64 // liquidity floor
	Workers      int     // parallelism
	SkipExisting bool    // skip dates that already have snapshots (non-destructive)
	EndDate      string  // optional end date (YYYY-MM-DD), defaults to latest warehouse date
	StartDate    string  // optional start date (YYYY-MM-DD), defaults to Days before EndDate
}

// BackfillSnapshots builds historical snapshots for the past N days.
// For each day, it loads bars up to that day, computes indicators, and stores the snapshot.
// This enables backtesting and audit trails.
// Uses BarCache for efficient bulk loading instead of per-symbol queries.
func BackfillSnapshots(ctx context.Context, loader BarLoader, dateLoader DateLoader, store SnapshotStore, symbols []string, opts BackfillOptions, log *slog.Logger) (int, error) {
	if opts.Days <= 0 && opts.StartDate == "" {
		return 0, nil
	}
	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	now := time.Now().UTC()
	var backfilled, skipped, failed int

	// Get latest date from warehouse with one query
	latestDate, err := dateLoader.LatestBarDate(ctx)
	if err != nil {
		log.Error("failed to get latest warehouse date", "error", err)
		return 0, err
	}
	
	// Determine start and end dates
	var endDate, startDate time.Time
	if opts.EndDate != "" {
		endDate, err = time.Parse("2006-01-02", opts.EndDate)
		if err != nil {
			log.Error("invalid end date format", "date", opts.EndDate, "error", err)
			return 0, err
		}
	} else {
		endDate = latestDate
	}
	
	if opts.StartDate != "" {
		startDate, err = time.Parse("2006-01-02", opts.StartDate)
		if err != nil {
			log.Error("invalid start date format", "date", opts.StartDate, "error", err)
			return 0, err
		}
	} else {
		startDate = endDate.AddDate(0, 0, -opts.Days+1)
	}
	
	// Report discrepancy if warehouse is stale (accounting for weekends)
	// Market data updates only happen on trading days after market close
	daysBehind := int(now.Sub(latestDate).Hours() / 24)
	if daysBehind > 3 { // More than 3 days means we're missing at least one trading day
		log.Warn("warehouse data is stale", "latest_date", latestDate.Format("2006-01-02"), "days_behind", daysBehind, "message", "pipeline needs to run to update warehouse")
	}
	
	log.Info("backfilling date range", "start", startDate.Format("2006-01-02"), "end", endDate.Format("2006-01-02"), "skip_existing", opts.SkipExisting)

	// Create SQLite-backed bar cache and bulk-load all bars once
	cache, err := NewBarCache(loader, log)
	if err != nil {
		log.Error("failed to create bar cache", "error", err)
		return 0, err
	}
	defer cache.Close()

	if err := cache.BulkLoad(ctx, symbols, startDate, endDate, 400); err != nil {
		log.Error("failed to bulk load bars", "error", err)
		return 0, err
	}

	// Calculate total days in range
	totalDays := int(endDate.Sub(startDate).Hours()/24) + 1
	
	// Collect all snapshots for batch writing
	var allSnapshots []snapshot.SnapshotBatch
	
	for dayOffset := totalDays - 1; dayOffset >= 0; dayOffset-- {
		if ctx.Err() != nil {
			return backfilled, ctx.Err()
		}
		snapshotDay := endDate.AddDate(0, 0, -dayOffset)
		
		// Skip weekends (Saturday=6, Sunday=0)
		// Market data only updates on trading days after market close
		if snapshotDay.Weekday() == time.Saturday || snapshotDay.Weekday() == time.Sunday {
			continue
		}
		
		snapshotDate := time.Date(snapshotDay.Year(), snapshotDay.Month(), snapshotDay.Day(), 0, 0, 0, 0, time.UTC).Unix()

		// Check if snapshot already exists (if SkipExisting is enabled)
		if opts.SkipExisting {
			exists, err := store.HasSnapshot(snapshotDate)
			if err != nil {
				log.Warn("failed to check existing snapshot", "date", snapshotDay.Format("2006-01-02"), "error", err)
			} else if exists {
				skipped++
				log.Debug("snapshot already exists, skipping", "date", snapshotDay.Format("2006-01-02"))
				continue
			}
		}

		// Generate snapshot using cached bars (no lookahead bias)
		res := UniverseFromCache(ctx, cache, symbols, snapshotDay, opts.MinDollarVol, workers, log)

		if len(res.Rows) == 0 {
			log.Warn("no data for snapshot", "date", snapshotDay.Format("2006-01-02"))
			failed++
			continue
		}

		// Collect snapshot for batch writing
		allSnapshots = append(allSnapshots, snapshot.SnapshotBatch{
			Rows:         res.Rows,
			BarTs:        res.Day.Unix(),
			SnapshotDate: snapshotDate,
		})
		backfilled++
		
		// Log progress every 10 snapshots
		if backfilled%10 == 0 {
			log.Info("backfill progress", "backfilled", backfilled, "skipped", skipped, "failed", failed, "total_days", totalDays, "current_date", snapshotDay.Format("2006-01-02"))
		}
	}

	// Write all snapshots in a single batch transaction
	if len(allSnapshots) > 0 {
		log.Info("writing snapshots in batch", "count", len(allSnapshots))
		if err := store.LoadHistoryBatch(allSnapshots); err != nil {
			log.Error("batch write failed", "error", err)
			return 0, err
		}
		log.Info("batch write complete", "snapshots_written", len(allSnapshots))
	}

	// Cleanup old snapshots (only if not skipping existing)
	if opts.KeepDays > 0 && !opts.SkipExisting {
		deleted, err := store.Cleanup(opts.KeepDays)
		if err != nil {
			log.Error("cleanup failed", "error", err)
		} else if deleted > 0 {
			log.Info("snapshots cleaned up", "deleted_dates", deleted, "keep_days", opts.KeepDays)
		}
	}

	log.Info("backfill complete", "backfilled", backfilled, "skipped", skipped, "failed", failed, "total_days", totalDays)
	return backfilled, nil
}
