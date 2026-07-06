// Package config loads scanner settings from the environment (env-first, stdlib
// only). As this grows it may adopt the pipeline's single-source-of-truth
// `settings` table pattern; for the first stab, simple env lookups suffice.
package config

import (
	"os"
	"strconv"
)

// Config holds all runtime settings.
type Config struct {
	// DBPath is the cetus warehouse SQLite file (opened READ-ONLY).
	DBPath string
	// SinceDays bounds how much recent history to load per symbol for the scan.
	SinceDays int
	// Lookback is the trailing bars forming the scan baseline.
	Lookback int
	// VolumeMult is the volume-breakout threshold (× trailing average).
	VolumeMult float64
	// GapPct is the gap threshold (fraction vs the prior close).
	GapPct float64
	// MaxSymbols caps the scanned universe (0 = no cap; handy for testing).
	MaxSymbols int

	// --- daily digest (the `digest` subcommand) ---

	// DigestLookbackDays bounds the history loaded per symbol; must cover the
	// longest window (252-bar 52-week high) plus slack.
	DigestLookbackDays int
	// MinDollarVol is the liquidity floor (close×volume) for the digest universe —
	// keeps feed-thin names out of the report.
	MinDollarVol float64
	// DigestTopN caps most sections; DigestMomentumN caps the momentum leaderboard.
	DigestTopN      int
	DigestMomentumN int
	// DigestFormat is one of html|text|json. DigestOut is the output file
	// ("" = stdout).
	DigestFormat string
	DigestOut    string
	// DigestWorkers sets scan parallelism (0 = runtime.NumCPU()).
	DigestWorkers int

	// --- serve (the `serve` subcommand: live dashboard over HTTP) ---

	// ServeAddr is the listen address for the dashboard server.
	ServeAddr string
	// ServeTTLSecs caches the rendered scan; a request older than this recomputes.
	ServeTTLSecs int
}

// defaultCetusDB is the fallback warehouse path. It stays pointed at the pipeline's
// working copy for now; it flips to the shared central store (../CETUS/cetus.db)
// once the DATA/CETUS migration lands. Prefer setting CETUS_DB.
const defaultCetusDB = "../cetus-marketdata-pipeline/cetus.db"

// resolveDBPath honors the shared warehouse convention:
//
//	SCANNER_DB_PATH (per-app override) → CETUS_DB (shared) → defaultCetusDB
//
// so one `CETUS_DB` env points every cetus consumer at the central store, while any
// single app can still override.
func resolveDBPath() string {
	if v := os.Getenv("SCANNER_DB_PATH"); v != "" {
		return v
	}
	if v := os.Getenv("CETUS_DB"); v != "" {
		return v
	}
	return defaultCetusDB
}

// Load reads configuration from the environment, applying lean defaults. The
// default DB path assumes the sibling layout under chartgeometry.com/DATA/.
func Load() Config {
	return Config{
		DBPath:     resolveDBPath(),
		SinceDays:  envInt("SCANNER_SINCE_DAYS", 120),
		Lookback:   envInt("SCANNER_LOOKBACK", 20),
		VolumeMult: envFloat("SCANNER_VOLUME_MULT", 2.0),
		GapPct:     envFloat("SCANNER_GAP_PCT", 0.05),
		MaxSymbols: envInt("SCANNER_MAX_SYMBOLS", 0),

		DigestLookbackDays: envInt("SCANNER_DIGEST_LOOKBACK_DAYS", 400),
		MinDollarVol:       envFloat("SCANNER_MIN_DOLLAR_VOL", 1e6),
		DigestTopN:         envInt("SCANNER_DIGEST_TOP_N", 8),
		DigestMomentumN:    envInt("SCANNER_DIGEST_MOMENTUM_N", 10),
		DigestFormat:       envOr("SCANNER_DIGEST_FORMAT", "html"),
		DigestOut:          envOr("SCANNER_DIGEST_OUT", ""),
		DigestWorkers:      envInt("SCANNER_DIGEST_WORKERS", 0),

		ServeAddr:    envOr("SCANNER_SERVE_ADDR", ":8080"),
		ServeTTLSecs: envInt("SCANNER_SERVE_TTL_SECS", 600),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
