// Command scanner reads the cetus warehouse (read-only) and detects signals.
//
// Two modes:
//
//	scanner            # default: stream per-symbol signals as JSONL on stdout
//	scanner scan       #   (explicit alias of the above)
//	scanner digest     # daily post-close digest (HTML|text|json) — the free-tier report
//
// Logs go to stderr so stdout carries only the output stream. Configuration is via
// environment variables (see README / docs/PHASE1_MVP.md).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"cetus-marketdata-scanner/internal/config"
	"cetus-marketdata-scanner/internal/dashboard"
	"cetus-marketdata-scanner/internal/digest"
	"cetus-marketdata-scanner/internal/scan"
	"cetus-marketdata-scanner/internal/scanner"
	"cetus-marketdata-scanner/internal/screen"
	"cetus-marketdata-scanner/internal/sentinel"
	"cetus-marketdata-scanner/internal/snapshot"
	"cetus-marketdata-scanner/internal/store"
	"cetus-marketdata-scanner/internal/study"
	"cetus-marketdata-scanner/internal/telemetry"
	"cetus-marketdata-scanner/internal/user"
)

func main() {
	log := telemetry.New(os.Stderr) // logs → stderr, output → stdout
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
		runStudies(ctx, log, cfg)
	default:
		log.Error("unknown subcommand", "arg", sub, "want", "scan|digest|serve|anomalies|studies")
		os.Exit(2)
	}
}

// openUniverse opens the warehouse read-only and returns the scannable symbols,
// capped by MaxSymbols. Callers own Close via the returned store.
func openUniverse(ctx context.Context, log *slog.Logger, cfg config.Config) (*store.Store, []string) {
	st, err := store.OpenReadOnly(ctx, cfg.DBPath)
	if err != nil {
		log.Error("open warehouse failed", "db", cfg.DBPath, "error", err)
		os.Exit(1)
	}
	universe, err := resolveUniverse(ctx, st, cfg)
	if err != nil {
		log.Error("resolve universe failed", "spec", cfg.Universe, "error", err)
		st.Close()
		os.Exit(1)
	}
	if cfg.MaxSymbols > 0 && len(universe) > cfg.MaxSymbols {
		universe = universe[:cfg.MaxSymbols]
	}
	log.Info("universe resolved", "spec", cfg.Universe, "symbols", len(universe))
	return st, universe
}

// resolveUniverse turns the SCANNER_UNIVERSE spec into a symbol list:
//
//	all              → every SUCCESS symbol (default)
//	exchange:NASDAQ  → SUCCESS symbols on that listing venue
//	list:sp500       → SUCCESS members of a symbol_lists watchlist
//	file:PATH        → SUCCESS symbols intersected with a tickers file (1/line, # comments)
func resolveUniverse(ctx context.Context, st *store.Store, cfg config.Config) ([]string, error) {
	spec := strings.TrimSpace(cfg.Universe)
	switch {
	case spec == "" || spec == "all":
		return st.Universe(ctx)
	case strings.HasPrefix(spec, "exchange:"):
		return st.UniverseExchange(ctx, strings.TrimPrefix(spec, "exchange:"))
	case strings.HasPrefix(spec, "list:"):
		return st.UniverseList(ctx, strings.TrimPrefix(spec, "list:"))
	case strings.HasPrefix(spec, "file:"):
		return universeFromFile(ctx, st, strings.TrimPrefix(spec, "file:"))
	default:
		return nil, fmt.Errorf("bad SCANNER_UNIVERSE %q (want all|exchange:X|list:Y|file:PATH)", spec)
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

// scanToDigest runs the concurrent universe scan and assembles the digest. Shared
// by the CLI digest and the serve dashboard so both compute identically.
func scanToDigest(ctx context.Context, log *slog.Logger, st *store.Store, universe []string, cfg config.Config) (digest.Digest, scan.Result) {
	since := time.Now().UTC().AddDate(0, 0, -cfg.DigestLookbackDays).Unix()
	res := scan.Universe(ctx, st, universe, scan.Options{
		Since:        since,
		MinDollarVol: cfg.MinDollarVol,
		Workers:      cfg.DigestWorkers,
	}, log)
	presets := screen.MVPPresets(cfg.DigestTopN, cfg.DigestMomentumN)
	return digest.Build(res.Day, len(res.Rows), res.Rows, presets), res
}

// runDigest builds the daily post-close digest and renders it to stdout or a file.
func runDigest(ctx context.Context, log *slog.Logger, cfg config.Config) {
	st, universe := openUniverse(ctx, log, cfg)
	defer st.Close()

	log.Info("digest starting", "db", cfg.DBPath, "symbols", len(universe),
		"lookback_days", cfg.DigestLookbackDays, "min_dollar_vol", cfg.MinDollarVol,
		"workers", cfg.DigestWorkers)

	d, res := scanToDigest(ctx, log, st, universe, cfg)

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
func runStudies(ctx context.Context, log *slog.Logger, cfg config.Config) {
	st, universe := openUniverse(ctx, log, cfg)
	defer st.Close()

	all, err := study.LoadFile(cfg.StudiesPath)
	if err != nil {
		log.Error("load studies failed", "path", cfg.StudiesPath, "error", err)
		os.Exit(1)
	}
	// Acting user (global for now); tier gates which studies are accessible.
	u := user.Global()
	u.ID = cfg.User
	studies := study.Accessible(all, u)

	since := time.Now().UTC().AddDate(0, 0, -cfg.DigestLookbackDays).Unix()
	res := scan.Universe(ctx, st, universe, scan.Options{
		Since: since, MinDollarVol: 0, Workers: cfg.DigestWorkers, // studies decide liquidity
	}, log)

	// The scanner's OWN store — never the cetus warehouse.
	snap, err := snapshot.Open(cfg.StoreDB)
	if err != nil {
		log.Error("open snapshot store failed", "store", cfg.StoreDB, "error", err)
		os.Exit(1)
	}
	defer snap.Close()
	if err := snap.Load(res.Rows, res.Day.Unix()); err != nil {
		log.Error("materialize snapshot failed", "error", err)
		os.Exit(1)
	}

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
	case "text":
		fmt.Printf("STUDIES — user %s (%s) — %s — snapshot %d symbols · store %s\n\n",
			u.ID, u.Tier, res.Day.Format("2006-01-02"), len(res.Rows), cfg.StoreDB)
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
		log.Error("unknown studies format", "format", cfg.StudiesFormat, "want", "text|jsonl")
		os.Exit(2)
	}
	log.Info("studies complete", "user", u.ID, "tier", u.Tier, "studies", len(studies),
		"snapshot_rows", len(res.Rows), "store", cfg.StoreDB, "day", res.Day.Format("2006-01-02"))
}

// runServe serves the digest as a live HTML dashboard, recomputing at most once per
// TTL (or on ?refresh=1). Listen address is SCANNER_SERVE_ADDR (default :8080).
func runServe(ctx context.Context, log *slog.Logger, cfg config.Config) {
	st, err := store.OpenReadOnly(ctx, cfg.DBPath)
	if err != nil {
		log.Error("open warehouse failed", "db", cfg.DBPath, "error", err)
		os.Exit(1)
	}
	defer st.Close()

	ttl := time.Duration(cfg.ServeTTLSecs) * time.Second
	var mu sync.Mutex
	var cached []byte
	var cachedAt time.Time

	render := func() ([]byte, error) {
		start := time.Now()
		universe, err := resolveUniverse(ctx, st, cfg)
		if err != nil {
			return nil, err
		}
		if cfg.MaxSymbols > 0 && len(universe) > cfg.MaxSymbols {
			universe = universe[:cfg.MaxSymbols]
		}
		since := time.Now().UTC().AddDate(0, 0, -cfg.DigestLookbackDays).Unix()
		// One scan, no floor: keep every row so the data-quality watch sees thin names;
		// the digest sections use the liquid subset.
		res := scan.Universe(ctx, st, universe, scan.Options{
			Since: since, MinDollarVol: 0, Workers: cfg.DigestWorkers,
		}, log)
		liquid := make([]screen.SnapshotRow, 0, len(res.Rows))
		for _, r := range res.Rows {
			if r.DollarVol >= cfg.MinDollarVol {
				liquid = append(liquid, r)
			}
		}
		presets := screen.MVPPresets(cfg.DigestTopN, cfg.DigestMomentumN)
		d := digest.Build(res.Day, len(liquid), liquid, presets)
		flags := sentinel.Tier0(res.Rows, sentinel.DefaultTier0())
		suspect, watch := sentinel.Counts(flags)

		stats, err := st.Stats(ctx)
		if err != nil {
			return nil, err
		}
		var size int64
		if fi, e := os.Stat(cfg.DBPath); e == nil {
			size = fi.Size()
		}

		m := dashboard.Model{
			Stats: stats, DBSizeBytes: size, ScanMillis: time.Since(start).Milliseconds(),
			Digest: d, Flags: flags, Suspect: suspect, Watch: watch,
		}
		var buf bytes.Buffer
		if err := m.HTML(&buf); err != nil {
			return nil, err
		}
		log.Info("dashboard rendered", "eligible", len(liquid), "flagged", len(flags),
			"day", res.Day.Format("2006-01-02"))
		return buf.Bytes(), nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		fresh := cached != nil && time.Since(cachedAt) < ttl && r.URL.Query().Get("refresh") == ""
		if !fresh {
			b, err := render()
			if err != nil {
				mu.Unlock()
				log.Error("render dashboard failed", "error", err)
				http.Error(w, "scan failed", http.StatusInternalServerError)
				return
			}
			cached, cachedAt = b, time.Now()
		}
		body := cached
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(body)
	})

	srv := &http.Server{Addr: cfg.ServeAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shCtx)
	}()

	log.Info("dashboard serving", "addr", cfg.ServeAddr, "db", cfg.DBPath, "ttl_secs", cfg.ServeTTLSecs)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("serve failed", "error", err)
		os.Exit(1)
	}
	log.Info("dashboard stopped")
}
