package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cetus-marketdata-scanner/internal/backtest"
	"cetus-marketdata-scanner/internal/config"
	"cetus-marketdata-scanner/internal/digest"
	"cetus-marketdata-scanner/internal/scan"
	"cetus-marketdata-scanner/internal/scanner"
	"cetus-marketdata-scanner/internal/sentinel"
	"cetus-marketdata-scanner/internal/serve"
	"cetus-marketdata-scanner/internal/snapshot"
	"cetus-marketdata-scanner/internal/store"
	"cetus-marketdata-scanner/internal/study"
	"cetus-marketdata-scanner/internal/telemetry"
	"cetus-marketdata-scanner/internal/user"
)

// studyQuota returns how many studies a user may own (0 = unlimited). Admins and
// pro users are unlimited; free users are capped by config.
func studyQuota(u user.User, cfg config.Config) int {
	if u.IsAdmin() || u.Tier == user.TierPro {
		return 0
	}
	return cfg.FreeStudyQuota
}

func main() {
	log := telemetry.New(os.Stderr) // logs → stderr, output → stdout
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Parse flags
	var studyFile string
	var outputFormat string
	var forceRescan bool
	var replayDate string
	if len(os.Args) > 1 {
		for i := 1; i < len(os.Args); i++ {
			if os.Args[i] == "--study" && i+1 < len(os.Args) {
				studyFile = os.Args[i+1]
				// Remove --study and its value from args
				os.Args = append(os.Args[:i], os.Args[i+2:]...)
				i-- // Adjust index since we removed elements
			} else if os.Args[i] == "--format" && i+1 < len(os.Args) {
				outputFormat = os.Args[i+1]
				// Remove --format and its value from args
				os.Args = append(os.Args[:i], os.Args[i+2:]...)
				i-- // Adjust index since we removed elements
			} else if os.Args[i] == "--force" {
				forceRescan = true
				// Remove --force from args
				os.Args = append(os.Args[:i], os.Args[i+1:]...)
				i-- // Adjust index since we removed elements
			} else if os.Args[i] == "--date" && i+1 < len(os.Args) {
				replayDate = os.Args[i+1]
				// Remove --date and its value from args
				os.Args = append(os.Args[:i], os.Args[i+2:]...)
				i-- // Adjust index since we removed elements
			}
		}
	}

	// Override config format if flag provided
	if outputFormat != "" {
		cfg.StudiesFormat = outputFormat
	}

	sub := ""
	if len(os.Args) > 1 {
		sub = os.Args[1]
	}
	switch sub {
	case "", "scan":
		runScan(ctx, log, cfg)
	case "digest":
		runDigest(ctx, log, cfg)
	case "serve":
		runServe(ctx, log, cfg)
	case "anomalies":
		runAnomalies(ctx, log, cfg)
	case "studies":
		runStudies(ctx, log, cfg, studyFile, forceRescan)
	case "users":
		runUsers(log, cfg)
	case "backfill":
		runBackfill(ctx, log, cfg)
	case "replay":
		runReplay(ctx, log, cfg, replayDate)
	default:
		log.Error("unknown subcommand", "arg", sub, "want", "scan|digest|serve|anomalies|studies|users|backfill|replay")
		os.Exit(2)
	}
}

// actingUser resolves the SCANNER_USER id against the registry. Falls back to the
// Global superuser for "global", or a plain free user for an unknown id.
func actingUser(log *slog.Logger, cfg config.Config) user.User {
	if reg, err := user.LoadFile(cfg.UsersPath); err == nil {
		if u, ok := reg.Find(cfg.User); ok {
			return u
		}
	} else {
		log.Warn("users registry not loaded; using defaults", "path", cfg.UsersPath, "error", err)
	}
	if cfg.User == "" || cfg.User == user.GlobalID {
		return user.Global()
	}
	log.Warn("acting user not in registry; treating as free", "user", cfg.User)
	return user.User{ID: cfg.User, Name: cfg.User, Tier: user.TierFree, Role: user.RoleUser}
}

// runUsers lists the seeded users.
func runUsers(log *slog.Logger, cfg config.Config) {
	reg, err := user.LoadFile(cfg.UsersPath)
	if err != nil {
		log.Error("load users failed", "path", cfg.UsersPath, "error", err)
		os.Exit(1)
	}
	fmt.Printf("USERS (%s)\n", cfg.UsersPath)
	for _, u := range reg.All() {
		fmt.Printf("  %-8s %-10s tier=%-4s role=%s\n", u.ID, u.Name, u.Tier, u.Role)
	}
}

// openUniverse opens the warehouse read-only and returns the scannable symbols,
// capped by MaxSymbols. Callers own Close via the returned store.
func openUniverse(ctx context.Context, log *slog.Logger, cfg config.Config) (*store.Store, []string) {
	st, err := store.OpenReadOnly(ctx, cfg.DBPath, cfg.SQLiteBusyTimeoutMS)
	if err != nil {
		log.Error("open warehouse failed", "db", cfg.DBPath, "error", err)
		os.Exit(1)
	}
	if err := st.CheckSchema(); err != nil {
		log.Error("schema version mismatch", "error", err)
		st.Close()
		os.Exit(1)
	}
	if st.SchemaVersion() == 0 {
		log.Warn("warehouse is unversioned — assuming compatibility", "db", cfg.DBPath)
	} else {
		log.Info("warehouse schema", "version", st.SchemaVersion())
	}
	universe, err := resolveUniverse(ctx, log, st, cfg)
	if err != nil {
		log.Error("resolve universe failed", "spec", cfg.Universe, "error", err)
		st.Close()
		os.Exit(1)
	}
	if cfg.MaxSymbols > 0 && len(universe) > cfg.MaxSymbols {
		universe = universe[:cfg.MaxSymbols]
	}
	log.Info("universe resolved", "spec", cfg.Universe, "symbols", len(universe), "bars", st.BarsTable())
	return st, universe
}

// resolveUniverse turns the SCANNER_UNIVERSE spec into a symbol list:
//
//	all              → every SUCCESS symbol
//	common           → SUCCESS common stock (drops warrants/units/rights/ETFs)
//	index:r3000      → SUCCESS common-stock members of an index (index_membership)
//	exchange:NASDAQ  → SUCCESS common stock on that listing venue
//	file:PATH        → SUCCESS symbols intersected with a tickers file (1/line, # comments)
//	list:CODE        → alias of index:CODE (back-compat)
//
// An index scope that resolves to 0 (index not yet seeded) falls back to `common`,
// so the default `index:r3000` works today and auto-scopes once the pipeline seeds.
func resolveUniverse(ctx context.Context, log *slog.Logger, st *store.Store, cfg config.Config) ([]string, error) {
	spec := strings.TrimSpace(cfg.Universe)
	switch {
	case spec == "" || spec == "all":
		return st.Universe(ctx)
	case spec == "common":
		return st.UniverseCommon(ctx)
	case strings.HasPrefix(spec, "exchange:"):
		return st.UniverseExchange(ctx, strings.TrimPrefix(spec, "exchange:"))
	case strings.HasPrefix(spec, "index:"), strings.HasPrefix(spec, "list:"):
		code := strings.TrimPrefix(strings.TrimPrefix(spec, "index:"), "list:")
		syms, err := st.UniverseIndex(ctx, code)
		if err != nil {
			return nil, err
		}
		if len(syms) == 0 {
			log.Warn("index not seeded — falling back to common stock", "index", code)
			return st.UniverseCommon(ctx)
		}
		return syms, nil
	case strings.HasPrefix(spec, "file:"):
		return universeFromFile(ctx, st, strings.TrimPrefix(spec, "file:"))
	default:
		return nil, fmt.Errorf("bad SCANNER_UNIVERSE %q (want all|common|index:X|exchange:X|file:PATH)", spec)
	}
}

// universeFromFile intersects a tickers file with the SUCCESS universe, so we only
// scan requested symbols that actually have data.
func universeFromFile(ctx context.Context, st *store.Store, path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read universe file: %w", err)
	}
	want := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.ToUpper(strings.TrimSpace(line))
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		want[t] = true
	}
	all, err := st.Universe(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(want))
	for _, sym := range all {
		if want[sym] {
			out = append(out, sym)
		}
	}
	return out, nil
}

// runScan is the original behaviour: per-symbol JSONL signal stream.
func runScan(ctx context.Context, log *slog.Logger, cfg config.Config) {
	st, universe := openUniverse(ctx, log, cfg)
	defer st.Close()

	since := time.Now().UTC().AddDate(0, 0, -cfg.SinceDays).Unix()
	scanCfg := scanner.Config{Lookback: cfg.Lookback, VolumeMult: cfg.VolumeMult, GapPct: cfg.GapPct}
	log.Info("scan starting", "db", cfg.DBPath, "symbols", len(universe),
		"lookback", cfg.Lookback, "since_days", cfg.SinceDays)

	enc := json.NewEncoder(os.Stdout)
	var scanned, signals int
	for _, sym := range universe {
		if ctx.Err() != nil {
			break
		}
		bars, err := st.LoadAdjustedBars(ctx, sym, since)
		if err != nil {
			log.Error("load bars failed", "symbol", sym, "error", err)
			continue
		}
		scanned++
		for _, sig := range scanner.Scan(sym, bars, scanCfg) {
			if err := enc.Encode(sig); err != nil {
				log.Error("encode signal failed", "error", err)
			}
			signals++
		}
	}
	log.Info("scan complete", "scanned", scanned, "signals", signals)
}

// scanAndBuild scans the universe, materializes the snapshot into the scanner's own
// store, and assembles the digest from the acting user's tier-accessible studies.
// Shared by the CLI digest and the serve dashboard so both are study-driven.
func scanAndBuild(ctx context.Context, log *slog.Logger, st *store.Store, universe []string, cfg config.Config) (digest.Digest, scan.Result, error) {
	since := time.Now().UTC().AddDate(0, 0, -cfg.DigestLookbackDays).Unix()
	res := scan.Universe(ctx, st, universe, scan.Options{
		Since: since, MinDollarVol: 0, Workers: cfg.DigestWorkers, // studies set their own liquidity in SQL
	}, log)

	snap, err := snapshot.Open(cfg.StoreDB, log)
	if err != nil {
		return digest.Digest{}, res, fmt.Errorf("open store %q: %w", cfg.StoreDB, err)
	}
	defer snap.Close()
	if err := snap.Load(res.Rows, res.Day.Unix()); err != nil {
		return digest.Digest{}, res, fmt.Errorf("materialize snapshot: %w", err)
	}

	all, err := study.LoadFile(cfg.StudiesPath)
	if err != nil {
		return digest.Digest{}, res, fmt.Errorf("load studies %q: %w", cfg.StudiesPath, err)
	}
	u := actingUser(log, cfg)
	d, err := digest.FromStudies(res.Day, res.Rows, snap, study.Accessible(all, u))
	return d, res, err
}

// runDigest builds the daily post-close digest and renders it to stdout or a file.
func runDigest(ctx context.Context, log *slog.Logger, cfg config.Config) {
	st, universe := openUniverse(ctx, log, cfg)
	defer st.Close()

	log.Info("digest starting", "db", cfg.DBPath, "symbols", len(universe),
		"lookback_days", cfg.DigestLookbackDays, "studies", cfg.StudiesPath, "user", cfg.User)

	d, res, err := scanAndBuild(ctx, log, st, universe, cfg)
	if err != nil {
		log.Error("build digest failed", "error", err)
		os.Exit(1)
	}

	out := os.Stdout
	if cfg.DigestOut != "" {
		f, err := os.Create(cfg.DigestOut)
		if err != nil {
			log.Error("create digest output failed", "path", cfg.DigestOut, "error", err)
			os.Exit(1)
		}
		defer f.Close()
		out = f
	}

	var rerr error
	switch cfg.DigestFormat {
	case "html":
		rerr = d.HTML(out)
	case "text":
		rerr = d.Text(out)
	case "json":
		rerr = d.JSON(out)
	default:
		log.Error("unknown digest format", "format", cfg.DigestFormat, "want", "html|text|json")
		os.Exit(2)
	}
	if rerr != nil {
		log.Error("render digest failed", "error", rerr)
		os.Exit(1)
	}
	log.Info("digest complete", "eligible", len(res.Rows), "scanned", res.Scanned,
		"day", res.Day.Format("2006-01-02"), "format", cfg.DigestFormat)
}

// runAnomalies runs the Sentinel Tier-0 data-quality pass over the selected
// universe (no liquidity floor — thin names are exactly what we want to catch) and
// reports flagged rows as text or JSONL.
func runAnomalies(ctx context.Context, log *slog.Logger, cfg config.Config) {
	st, universe := openUniverse(ctx, log, cfg)
	defer st.Close()

	since := time.Now().UTC().AddDate(0, 0, -cfg.DigestLookbackDays).Unix()
	res := scan.Universe(ctx, st, universe, scan.Options{
		Since:        since,
		MinDollarVol: 0, // keep every row; the point is to surface the thin/extreme ones
		Workers:      cfg.DigestWorkers,
	}, log)

	flags := sentinel.Tier0(res.Rows, sentinel.DefaultTier0())
	suspect, watch := sentinel.Counts(flags)

	switch cfg.AnomalyFormat {
	case "jsonl":
		enc := json.NewEncoder(os.Stdout)
		for _, f := range flags {
			if err := enc.Encode(f); err != nil {
				log.Error("encode anomaly failed", "error", err)
			}
		}
	case "text":
		fmt.Printf("DATA-QUALITY ANOMALIES — %s — %d flagged (%d suspect, %d watch) of %d scanned\n\n",
			res.Day.Format("2006-01-02"), len(flags), suspect, watch, len(res.Rows))
		for _, f := range flags {
			ratio := "—"
			if !math.IsNaN(f.Ratio200) {
				ratio = fmt.Sprintf("%.2fx", f.Ratio200)
			}
			fmt.Printf("  [%-7s] %-8s  3mo %+6.0f%%   $vol %6.1fM   px/200dma %-6s  %s\n",
				strings.ToUpper(string(f.Severity)), f.Symbol, f.Ret3m*100, f.DollarVol/1e6, ratio, f.Reason)
		}
	default:
		log.Error("unknown anomaly format", "format", cfg.AnomalyFormat, "want", "text|jsonl")
		os.Exit(2)
	}
	log.Info("anomalies complete", "flagged", len(flags), "suspect", suspect, "watch", watch,
		"scanned", len(res.Rows), "day", res.Day.Format("2006-01-02"))
}

// runStudies materializes the snapshot into the scanner's OWN store (in-memory by
// default) and runs the acting user's tier-accessible SQL-WHERE studies against it.
// If a recent snapshot exists in the store, it reuses it instead of rescanning.
func runStudies(ctx context.Context, log *slog.Logger, cfg config.Config, studyFile string, forceRescan bool) {
	// Use provided study file or fall back to config
	studiesPath := cfg.StudiesPath
	if studyFile != "" {
		studiesPath = studyFile
	}

	all, err := study.LoadFile(studiesPath)
	if err != nil {
		log.Error("load studies failed", "path", studiesPath, "error", err)
		os.Exit(1)
	}
	// Acting user resolved from the registry; tier/role gate which studies run.
	u := actingUser(log, cfg)
	studies := study.Accessible(all, u)

	// The scanner's OWN store — never the cetus warehouse.
	snap, err := snapshot.Open(cfg.StoreDB, log)
	if err != nil {
		log.Error("open snapshot store failed", "store", cfg.StoreDB, "error", err)
		os.Exit(1)
	}
	defer snap.Close()

	// Check if we have a recent snapshot (within last 24 hours)
	t0 := time.Now()
	dates, err := snap.ListSnapshots()
	if err != nil {
		// Table doesn't exist yet - that's fine, we'll create it with a full scan
		log.Debug("no existing snapshots found", "error", err)
		dates = nil
	}

	var snapshotDate int64
	var symbolsScanned, symbolsEligible int
	var scanDuration time.Duration

	// Use existing snapshot if available (unless --force is set)
	if len(dates) > 0 && !forceRescan {
		latestDate := dates[0] // ListSnapshots returns newest first
		age := time.Since(time.Unix(latestDate, 0))
		
		// Always reuse the latest available snapshot
		snapshotTime := time.Unix(latestDate, 0)
		log.Info("reusing existing snapshot", "date", snapshotTime.Format("2006-01-02"), "age", age.Round(time.Hour))
		
		// Warn if snapshot is stale (more than 1 day old)
		if age > 24*time.Hour {
			daysOld := int(age.Hours() / 24)
			fmt.Printf("\n⚠️  WARNING: Snapshot is %d days old (latest: %s)\n", daysOld, snapshotTime.Format("2006-01-02"))
			fmt.Printf("   Run the pipeline to update warehouse data, then use --force to rebuild snapshot\n\n")
		}
		
		if err := snap.SetActive(latestDate); err != nil {
			log.Error("set active snapshot failed", "error", err)
			os.Exit(1)
		}
		snapshotDate = latestDate
		// We don't know exact counts without scanning, but we can estimate
		symbolsScanned = 0 // Unknown for cached snapshot
		symbolsEligible = 0 // Unknown for cached snapshot
		scanDuration = 0 // No scan needed
	}

	// If no recent snapshot, do a full scan
	if snapshotDate == 0 {
		st, universe := openUniverse(ctx, log, cfg)
		defer st.Close()

		since := time.Now().UTC().AddDate(0, 0, -cfg.DigestLookbackDays).Unix()
		
		// Phase 1: Load bars + compute indicators
		res := scan.Universe(ctx, st, universe, scan.Options{
			Since: since, MinDollarVol: 0, Workers: cfg.DigestWorkers,
		}, log)
		t1 := time.Now()
		log.Info("scan complete", "symbols", len(res.Rows), "duration", t1.Sub(t0).String())

		// Phase 2: Materialize snapshot into SQLite
		t2 := time.Now()
		if err := snap.Load(res.Rows, res.Day.Unix()); err != nil {
			log.Error("materialize snapshot failed", "error", err)
			os.Exit(1)
		}
		t3 := time.Now()
		log.Info("snapshot materialized", "rows", len(res.Rows), "duration", t3.Sub(t2).String())
		
		snapshotDate = res.Day.Unix()
		symbolsScanned = res.Scanned
		symbolsEligible = len(res.Rows)
		scanDuration = t3.Sub(t0)
	}

	totalDuration := time.Since(t0)

	// Display execution metadata header
	fmt.Printf("\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  STUDY EXECUTION METADATA\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  User:              %s (%s)\n", u.ID, u.Tier)
	fmt.Printf("  Studies File:      %s\n", studiesPath)
	fmt.Printf("  Studies Loaded:    %d (accessible: %d)\n", len(all), len(studies))
	fmt.Printf("  Snapshot Date:     %s\n", time.Unix(snapshotDate, 0).Format("2006-01-02"))
	if symbolsScanned > 0 {
		fmt.Printf("  Symbols Scanned:   %d\n", symbolsScanned)
		fmt.Printf("  Symbols Eligible:  %d\n", symbolsEligible)
	} else {
		fmt.Printf("  Symbols:           (cached snapshot)\n")
	}
	fmt.Printf("  Store:             %s\n", cfg.StoreDB)
	if scanDuration > 0 {
		fmt.Printf("  Scan Duration:     %s\n", scanDuration.Round(time.Millisecond))
	} else {
		fmt.Printf("  Scan Duration:     (skipped - using cached)\n")
	}
	fmt.Printf("  Total Duration:    %s\n", totalDuration.Round(time.Millisecond))
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("\n")

	switch cfg.StudiesFormat {
	case "jsonl":
		enc := json.NewEncoder(os.Stdout)
		for _, s := range studies {
			matches, err := snap.Run(s)
			if err != nil {
				log.Error("run study failed", "study", s.Key, "error", err)
				continue
			}
			for _, m := range matches {
				_ = enc.Encode(struct {
					Owner string `json:"owner"`
					Study string `json:"study"`
					snapshot.Match
				}{s.Owner, s.Key, m})
			}
		}
	case "json":
		// Rich JSON output with metadata
		type StudyResult struct {
			Study  study.Study       `json:"study"`
			Count  int               `json:"count"`
			Matches []snapshot.Match `json:"matches"`
		}
		
		type Output struct {
			Metadata struct {
				User            string `json:"user"`
				Tier            string `json:"tier"`
				SnapshotDate    string `json:"snapshot_date"`
				SymbolsScanned  int    `json:"symbols_scanned"`
				SymbolsEligible int    `json:"symbols_eligible"`
				StudiesFile     string `json:"studies_file"`
				StudiesLoaded   int    `json:"studies_loaded"`
				StudiesRun      int    `json:"studies_run"`
				ScanDuration    string `json:"scan_duration"`
				TotalDuration   string `json:"total_duration"`
			} `json:"metadata"`
			Results []StudyResult `json:"results"`
		}
		
		output := Output{}
		output.Metadata.User = u.ID
		output.Metadata.Tier = string(u.Tier)
		output.Metadata.SnapshotDate = time.Unix(snapshotDate, 0).Format("2006-01-02")
		output.Metadata.SymbolsScanned = symbolsScanned
		output.Metadata.SymbolsEligible = symbolsEligible
		output.Metadata.StudiesFile = studiesPath
		output.Metadata.StudiesLoaded = len(all)
		output.Metadata.StudiesRun = len(studies)
		if scanDuration > 0 {
			output.Metadata.ScanDuration = scanDuration.Round(time.Millisecond).String()
		} else {
			output.Metadata.ScanDuration = "0s (cached)"
		}
		output.Metadata.TotalDuration = totalDuration.Round(time.Millisecond).String()
		
		for _, s := range studies {
			matches, err := snap.Run(s)
			if err != nil {
				log.Error("run study failed", "study", s.Key, "error", err)
				continue
			}
			output.Results = append(output.Results, StudyResult{
				Study:   s,
				Count:   len(matches),
				Matches: matches,
			})
		}
		
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(output)
		
	case "csv":
		w := csv.NewWriter(os.Stdout)
		// Write header
		_ = w.Write([]string{
			"study_key", "study_title", "study_owner", "study_tier",
			"symbol", "close", "rsi14", "ret_3m", "dollar_vol",
		})
		
		for _, s := range studies {
			matches, err := snap.Run(s)
			if err != nil {
				log.Error("run study failed", "study", s.Key, "error", err)
				continue
			}
			for _, m := range matches {
				_ = w.Write([]string{
					s.Key,
					s.Title,
					s.Owner,
					string(s.Tier),
					m.Symbol,
					fmt.Sprintf("%.2f", m.Close),
					fmt.Sprintf("%.1f", m.RSI14),
					fmt.Sprintf("%.4f", m.Ret3m),
					fmt.Sprintf("%.0f", m.DollarVol),
				})
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			log.Error("csv write failed", "error", err)
			os.Exit(1)
		}
		
	case "text":
		fmt.Printf("STUDIES — user %s (%s) — %s — snapshot %d symbols · store %s\n\n",
			u.ID, u.Tier, time.Unix(snapshotDate, 0).Format("2006-01-02"), symbolsEligible, cfg.StoreDB)
		for _, s := range studies {
			matches, err := snap.Run(s)
			if err != nil {
				log.Error("run study failed", "study", s.Key, "error", err)
				continue
			}
			fmt.Printf("%s %s  [%s]  (%d)\n", s.Emoji, s.Title, s.Tier, len(matches))
			for _, m := range matches {
				fmt.Printf("   %-8s %9.2f  RSI %5.1f  3mo %+6.0f%%  $%.1fM\n",
					m.Symbol, m.Close, m.RSI14, m.Ret3m*100, m.DollarVol/1e6)
			}
			fmt.Println()
		}
	default:
		log.Error("unknown studies format", "format", cfg.StudiesFormat, "want", "text|jsonl|json|csv")
		os.Exit(2)
	}
	log.Info("studies complete", "user", u.ID, "tier", u.Tier, "studies", len(studies),
		"snapshot_rows", symbolsEligible, "store", cfg.StoreDB, "day", time.Unix(snapshotDate, 0).Format("2006-01-02"))
}

// runBackfill builds historical snapshots for the past N days and stores them
// with date-stamped retention. Requires SCANNER_STORE_DB to be a persistent path
// (not :memory:) so snapshots survive across runs.
func runBackfill(ctx context.Context, log *slog.Logger, cfg config.Config) {
	if cfg.StoreDB == "" || cfg.StoreDB == ":memory:" {
		log.Error("backfill requires a persistent SCANNER_STORE_DB (not :memory:)")
		os.Exit(1)
	}
	if cfg.BackfillDays <= 0 {
		log.Error("SCANNER_BACKFILL_DAYS must be > 0 for backfill")
		os.Exit(1)
	}

	st, universe := openUniverse(ctx, log, cfg)
	defer st.Close()

	snap, err := snapshot.Open(cfg.StoreDB, log)
	if err != nil {
		log.Error("open snapshot store failed", "store", cfg.StoreDB, "error", err)
		os.Exit(1)
	}
	defer snap.Close()

	log.Info("backfill starting", "days", cfg.BackfillDays, "retention_days", cfg.SnapshotRetentionDays,
		"symbols", len(universe), "store", cfg.StoreDB)

	backfilled, err := scan.BackfillSnapshots(ctx, st, st, snap, universe, scan.BackfillOptions{
		Days:         cfg.BackfillDays,
		KeepDays:     cfg.SnapshotRetentionDays,
		MinDollarVol: 0,
		Workers:      cfg.DigestWorkers,
	}, log)
	if err != nil {
		log.Error("backfill failed", "error", err)
		os.Exit(1)
	}

	dates, _ := snap.ListSnapshots()
	log.Info("backfill complete", "backfilled", backfilled, "total_snapshots", len(dates),
		"oldest", func() string {
			if len(dates) == 0 {
				return "none"
			}
			return time.Unix(dates[len(dates)-1], 0).UTC().Format("2006-01-02")
		}(),
		"newest", func() string {
			if len(dates) == 0 {
				return "none"
			}
			return time.Unix(dates[0], 0).UTC().Format("2006-01-02")
		}())
}

// runServe starts the HTTP dashboard server.
func runServe(ctx context.Context, log *slog.Logger, cfg config.Config) {
	srv, err := serve.New(ctx, log, cfg)
	if err != nil {
		log.Error("failed to create server", "error", err)
		os.Exit(1)
	}
	srv.Run()
}

// runReplay runs a historical scan and calculates P/L to current date.
func runReplay(ctx context.Context, log *slog.Logger, cfg config.Config, dateStr string) {
	if dateStr == "" {
		log.Error("--date flag required for replay")
		os.Exit(1)
	}

	scanDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		log.Error("invalid date format", "date", dateStr, "want", "YYYY-MM-DD")
		os.Exit(1)
	}

	st, universe := openUniverse(ctx, log, cfg)
	defer st.Close()

	snap, err := snapshot.Open(cfg.StoreDB, log)
	if err != nil {
		log.Error("open snapshot store failed", "store", cfg.StoreDB, "error", err)
		os.Exit(1)
	}
	defer snap.Close()

	scanCfg := scanner.Config{
		Lookback:   cfg.Lookback,
		VolumeMult: cfg.VolumeMult,
		GapPct:     cfg.GapPct,
	}

	log.Info("replay starting", "date", scanDate.Format("2006-01-02"), "symbols", len(universe))

	result, err := backtest.ReplayHistoricalScan(ctx, st, snap, universe, scanDate, scanCfg, log)
	if err != nil {
		log.Error("replay failed", "error", err)
		os.Exit(1)
	}

	// Output results
	fmt.Printf("\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  HISTORICAL SCAN REPLAY\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  Scan Date:    %s\n", result.ScanDate.Format("2006-01-02"))
	fmt.Printf("  Current Date: %s\n", result.CurrentDate.Format("2006-01-02"))
	fmt.Printf("  Symbols:      %d\n", len(universe))
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("\n")

	if len(result.Signals) == 0 {
		fmt.Printf("No signals found on %s\n", scanDate.Format("2006-01-02"))
		return
	}

	// Group signals by type
	byType := make(map[string][]backtest.HistoricalSignal)
	for _, sig := range result.Signals {
		byType[sig.Type] = append(byType[sig.Type], sig)
	}

	for sigType, signals := range byType {
		fmt.Printf("%s signals (%d):\n", strings.ToUpper(sigType), len(signals))
		for _, sig := range signals {
			retStr := fmt.Sprintf("%+.1f%%", sig.Return*100)
			profitStr := fmt.Sprintf("+%.1f%%", sig.MaxProfit*100)
			lossStr := fmt.Sprintf("%.1f%%", sig.MaxLoss*100)
			fmt.Printf("  %-8s  entry=$%.2f  current=$%.2f  %s  max:%s/%s\n",
				sig.Symbol, sig.EntryPx, sig.CurrentPx, retStr, profitStr, lossStr)
		}
		fmt.Printf("\n")
	}

	// Summary
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  SUMMARY\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  Total Signals: %d\n", result.Summary.TotalSignals)
	fmt.Printf("  Winners:       %d (%.1f%%)\n", result.Summary.Winners, result.Summary.WinRate*100)
	fmt.Printf("  Losers:        %d\n", result.Summary.Losers)
	fmt.Printf("  Avg Return:    %+.2f%%\n", result.Summary.AvgReturn*100)
	fmt.Printf("  Total Return:  %+.2f%%\n", result.Summary.TotalReturn*100)
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
}
