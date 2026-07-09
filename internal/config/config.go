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
	// Universe selects the scan set: "all" (default), "exchange:NASDAQ",
	// "list:sp500" (a symbol_lists watchlist), or "file:/path/to/tickers.txt".
	Universe string

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
	// SessionHours is how long a login session stays valid.
	SessionHours int
	// AuthMode is "login" (built-in sessions) or "proxy" (trust an identity header
	// injected by a reverse proxy such as caddy-security). TrustedUserHeader names
	// that header in proxy mode.
	AuthMode          string
	TrustedUserHeader string
	// JWT verification for proxy mode. When a key is set (HMAC secret or RSA pubkey
	// file), the identity is taken from a verified token instead of a raw header.
	JWTHeader     string // header carrying the token (Bearer prefix stripped)
	JWTJWKSURL    string // JWKS endpoint (rotating RSA keys, e.g. Cloudflare Access)
	JWTHMACSecret string // HS256/384/512 shared secret
	JWTPubKeyFile string // RS256/384/512 RSA public key (PEM)
	JWTUserClaim  string // claim holding the user id (default "sub")
	JWTIssuer     string // optional expected iss
	JWTAudience   string // optional expected aud

	// AnomalyFormat is the `anomalies` output: text (default) | jsonl.
	AnomalyFormat string

	// --- the scanner's OWN store (separate from the read-only cetus warehouse) ---

	// StoreDB is the scanner's own SQLite DB — it materializes the snapshot here and
	// (soon) holds users/studies/alerts. Rebuildable; never the cetus warehouse.
	// ":memory:" for an ephemeral snapshot.
	StoreDB string
	// StudiesPath is the JSONL of SQL-WHERE studies (owned by users, tier-gated).
	StudiesPath string
	// StudiesFormat is the `studies` output: text (default) | jsonl.
	StudiesFormat string
	// FreeStudyQuota caps how many studies a free-tier user may own (pro = unlimited).
	FreeStudyQuota int
	// UsersPath is the JSONL user registry (id, tier, role).
	UsersPath string
	// User is the acting user id — resolved against UsersPath; tier/role gate access.
	User string

	// --- snapshot history ---

	// SnapshotRetentionDays is how many days of snapshots to keep (0 = no retention).
	SnapshotRetentionDays int
	// BackfillDays is how many days to backfill on `scanner backfill` (0 = no backfill).
	BackfillDays int
}

// defaultCetusDB is the fallback warehouse path: the shared CENTRAL store at
// DATA/CETUS (../../../DATA/CETUS/cetus.db from the scanner dir), which the pipeline's
// create-cetus.sh --wipe publishes to. This is the canonical, latest warehouse — NOT
// the pipeline repo's local dev copy. Prefer setting CETUS_DB (absolute) for robustness
// against the run directory.
const defaultCetusDB = "../../../DATA/CETUS/cetus.db"

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
		// Downstream default is the Russell 3000 (the liquid, data-solid ~21% slice);
		// falls back to `common` until the pipeline seeds index_membership.
		Universe: envOr("SCANNER_UNIVERSE", "index:r3000"),

		DigestLookbackDays: envInt("SCANNER_DIGEST_LOOKBACK_DAYS", 400),
		MinDollarVol:       envFloat("SCANNER_MIN_DOLLAR_VOL", 1e6),
		DigestTopN:         envInt("SCANNER_DIGEST_TOP_N", 8),
		DigestMomentumN:    envInt("SCANNER_DIGEST_MOMENTUM_N", 10),
		DigestFormat:       envOr("SCANNER_DIGEST_FORMAT", "html"),
		DigestOut:          envOr("SCANNER_DIGEST_OUT", ""),
		DigestWorkers:      envInt("SCANNER_DIGEST_WORKERS", 0),

		ServeAddr:    envOr("SCANNER_SERVE_ADDR", ":8080"),
		ServeTTLSecs: envInt("SCANNER_SERVE_TTL_SECS", 600),
		SessionHours:      envInt("SCANNER_SESSION_HOURS", 8),
		AuthMode:          envOr("SCANNER_AUTH_MODE", "login"),
		TrustedUserHeader: envOr("SCANNER_TRUSTED_USER_HEADER", "X-Token-User"),
		JWTHeader:         envOr("SCANNER_JWT_HEADER", "Authorization"),
		JWTJWKSURL:        envOr("SCANNER_JWT_JWKS_URL", ""),
		JWTHMACSecret:     envOr("SCANNER_JWT_HMAC_SECRET", ""),
		JWTPubKeyFile:     envOr("SCANNER_JWT_PUBKEY_FILE", ""),
		JWTUserClaim:      envOr("SCANNER_JWT_USER_CLAIM", "sub"),
		JWTIssuer:         envOr("SCANNER_JWT_ISSUER", ""),
		JWTAudience:       envOr("SCANNER_JWT_AUDIENCE", ""),

		AnomalyFormat: envOr("SCANNER_ANOMALY_FORMAT", "text"),

		// Own store defaults to in-memory (rebuilt each run) — on a 32 GB box the
		// whole snapshot lives in RAM. Set a path (e.g. scanner.db) to persist it
		// for ad-hoc sqlite3 inspection.
		StoreDB:       envOr("SCANNER_STORE_DB", ":memory:"),
		StudiesPath:   envOr("SCANNER_STUDIES_PATH", "studies.jsonl"),
		StudiesFormat: envOr("SCANNER_STUDIES_FORMAT", "text"),
		FreeStudyQuota: envInt("SCANNER_FREE_STUDY_QUOTA", 3),
		UsersPath:      envOr("SCANNER_USERS_PATH", "users.jsonl"),
		User:           envOr("SCANNER_USER", "global"),

		// Snapshot history: keep 90 days by default, backfill 0 (opt-in).
		SnapshotRetentionDays: envInt("SCANNER_SNAPSHOT_RETENTION_DAYS", 90),
		BackfillDays:          envInt("SCANNER_BACKFILL_DAYS", 0),
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
