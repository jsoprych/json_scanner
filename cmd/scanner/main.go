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
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"cetus-marketdata-scanner/internal/config"
	"cetus-marketdata-scanner/internal/digest"
	"cetus-marketdata-scanner/internal/scan"
	"cetus-marketdata-scanner/internal/scanner"
	"cetus-marketdata-scanner/internal/screen"
	"cetus-marketdata-scanner/internal/store"
	"cetus-marketdata-scanner/internal/telemetry"
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
	default:
		log.Error("unknown subcommand", "arg", sub, "want", "scan|digest|serve")
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
	universe, err := st.Universe(ctx)
	if err != nil {
		log.Error("load universe failed", "error", err)
		st.Close()
		os.Exit(1)
	}
	if cfg.MaxSymbols > 0 && len(universe) > cfg.MaxSymbols {
		universe = universe[:cfg.MaxSymbols]
	}
	return st, universe
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
		universe, err := st.Universe(ctx)
		if err != nil {
			return nil, err
		}
		if cfg.MaxSymbols > 0 && len(universe) > cfg.MaxSymbols {
			universe = universe[:cfg.MaxSymbols]
		}
		d, res := scanToDigest(ctx, log, st, universe, cfg)
		var buf bytes.Buffer
		if err := d.HTML(&buf); err != nil {
			return nil, err
		}
		log.Info("dashboard rendered", "eligible", len(res.Rows), "day", res.Day.Format("2006-01-02"))
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
