// Package config loads scanner settings from the environment (env-first, stdlib
// only). As this grows it may adopt the pipeline's single-source-of-truth
// `settings` table pattern; for the first stab, simple env lookups suffice.
package config

import (
	"os"
	"path/filepath"
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
	JWTHeader         string // header carrying the token (Bearer prefix stripped)
	JWTJWKSURL        string // JWKS endpoint (rotating RSA keys, e.g. Cloudflare Access)
	JWTHMACSecret     string // HS256/384/512 shared secret (verify only)
	JWTPubKeyFile     string // RS256/384/512 RSA public key (PEM) (verify only)
	JWTUserClaim      string // claim holding the user id (default "sub")
	JWTIssuer         string // optional expected iss
	JWTAudience       string // optional expected aud
	JWTSignSecret     string // HS256 shared secret for issuing tokens (login endpoint)
	JWTSignTTLHours   int    // token lifetime in hours (default 24)

	// AnomalyFormat is the `anomalies` output: text (default) | jsonl.
	AnomalyFormat string

	// --- TLS / HTTPS ---

	// TLSDomain enables Let's Encrypt auto-HTTPS when set (e.g., "scan.chartgeometry.com").
	TLSDomain string
	// TLSCacheDir is where TLS certificates are cached (default: "certs").
	TLSCacheDir string

	// OpenSignup enables self-registration (default false).
	OpenSignup bool

	// --- Auth & Security ---

	LoginLockoutSecs    int    // Account lockout duration (default 900 = 15 min)
	LoginMaxFailures    int    // Max failed attempts before lockout (default 5)
	LoginRateWindowSecs int    // Login rate-limit window per IP (default 60)
	LoginRateMaxAttempts int   // Max login attempts per window per IP (default 5)
	BootstrapAdminPW    string // Default admin password on first run (default "admin")
	PasswordMinLength   int    // Minimum password length (default 8)
	PasswordRequireUpper bool  // Require uppercase in passwords (default true)
	PasswordRequireDigit bool  // Require digit/special in passwords (default true)
	UsernameMinLength   int    // Minimum username length (default 3)
	UsernameMaxLength   int    // Maximum username length (default 32)
	ResultLimitFree     int    // Results shown for free tier (default 26)
	ResultLimitPro      int    // Results shown for pro tier (default 101)

	// --- HTTP Server ---

	HTTPHeaderTimeoutSecs int // HTTP read header timeout (default 5)
	SQLiteBusyTimeoutMS   int // SQLite busy timeout ms (default 5000)
	JWTLeewaySecs         int // JWT clock leeway seconds (default 60)

	// --- NLP / LLM ---

	LLMBaseURL string // LLM API base URL (e.g., "https://api.deepseek.com/v1")
	LLMAPIKey  string // LLM API key
	LLMModel   string // LLM model name (e.g., "deepseek-chat")

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
	// SubscriptionsPath is the JSONL of user-study subscriptions.
	SubscriptionsPath string
	// User is the acting user id — resolved against UsersPath; tier/role gate access.
	User string

	// --- snapshot history ---

	// SnapshotRetentionDays is how many days of snapshots to keep (0 = no retention).
	SnapshotRetentionDays int
	// BackfillDays is how many days to backfill on `scanner backfill` (0 = no backfill).
	BackfillDays int
}

// defaultCetusDB is the fallback warehouse path, relative to the executable.
// From bin/scanner: ../../CETUS/cetus.db reaches chartgeometry.com/DATA/CETUS/cetus.db.
// Prefer setting CETUS_DB (absolute) for robustness.
const defaultCetusDB = "../../CETUS/cetus.db"

// defaultScannerDB is the fallback path for the scanner's own database,
// relative to the executable. From bin/scanner: ../../SCANNER/scanner.db.
// Prefer setting SCANNER_STORE_DB (absolute) for robustness.
const defaultScannerDB = "../../SCANNER/scanner.db"

// resolveRelative resolves a path relative to the executable's directory.
// If the path is already absolute, empty, or ":memory:", it's returned unchanged.
// If the executable can't be determined, the path is returned as-is (CWD-relative).
func resolveRelative(path string) string {
	if path == "" || path == ":memory:" || filepath.IsAbs(path) {
		return path
	}
	exe, err := os.Executable()
	if err != nil {
		return path
	}
	return filepath.Join(filepath.Dir(exe), path)
}

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
	return resolveRelative(defaultCetusDB)
}

// resolveStoreDB returns the scanner's own store path. Env var values are used
// verbatim (user controls absolute vs CWD-relative); only the built-in default
// is resolved relative to the executable.
func resolveStoreDB() string {
	if v := os.Getenv("SCANNER_STORE_DB"); v != "" {
		return v
	}
	return resolveRelative(defaultScannerDB)
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
		JWTSignSecret:     envOr("SCANNER_JWT_SIGN_SECRET", ""),
		JWTSignTTLHours:   envInt("SCANNER_JWT_SIGN_TTL_HOURS", 24),

		AnomalyFormat: envOr("SCANNER_ANOMALY_FORMAT", "text"),

		// TLS
		TLSDomain:   envOr("SCANNER_TLS_DOMAIN", ""),
		TLSCacheDir: envOr("SCANNER_TLS_CACHE_DIR", "certs"),
		OpenSignup:  envOr("SCANNER_OPEN_SIGNUP", "") == "true",

		// Auth & Security
		LoginLockoutSecs:     envInt("SCANNER_LOGIN_LOCKOUT_SECS", 900),
		LoginMaxFailures:     envInt("SCANNER_LOGIN_MAX_FAILURES", 5),
		LoginRateWindowSecs:  envInt("SCANNER_LOGIN_RATE_WINDOW_SECS", 60),
		LoginRateMaxAttempts: envInt("SCANNER_LOGIN_RATE_MAX_ATTEMPTS", 5),
		BootstrapAdminPW:     envOr("SCANNER_BOOTSTRAP_ADMIN_PW", "admin"),
		PasswordMinLength:    envInt("SCANNER_PW_MIN_LENGTH", 8),
		PasswordRequireUpper: envOr("SCANNER_PW_REQUIRE_UPPER", "true") == "true",
		PasswordRequireDigit: envOr("SCANNER_PW_REQUIRE_DIGIT", "true") == "true",
		UsernameMinLength:    envInt("SCANNER_USERNAME_MIN_LENGTH", 3),
		UsernameMaxLength:    envInt("SCANNER_USERNAME_MAX_LENGTH", 32),
		ResultLimitFree:      envInt("SCANNER_RESULT_LIMIT_FREE", 26),
		ResultLimitPro:       envInt("SCANNER_RESULT_LIMIT_PRO", 101),

		// HTTP Server
		HTTPHeaderTimeoutSecs: envInt("SCANNER_HTTP_HEADER_TIMEOUT_SECS", 5),
		SQLiteBusyTimeoutMS:   envInt("SCANNER_SQLITE_BUSY_TIMEOUT_MS", 5000),
		JWTLeewaySecs:         envInt("SCANNER_JWT_LEEWAY_SECS", 60),

		// NLP
		LLMBaseURL: envOr("SCANNER_LLM_BASE_URL", "https://api.deepseek.com/v1"),
		LLMAPIKey:  envOr("SCANNER_LLM_API_KEY", ""),
		LLMModel:   envOr("SCANNER_LLM_MODEL", "deepseek-chat"),

		// Own store defaults to persistent path at DATA/SCANNER/scanner.db,
		// resolved relative to the executable. Set SCANNER_STORE_DB to override,
		// or use ":memory:" for ephemeral snapshots.
		StoreDB:       resolveStoreDB(),
		StudiesPath:   envOr("SCANNER_STUDIES_PATH", "studies.jsonl"),
		StudiesFormat: envOr("SCANNER_STUDIES_FORMAT", "text"),
		FreeStudyQuota: envInt("SCANNER_FREE_STUDY_QUOTA", 3),
		UsersPath:      envOr("SCANNER_USERS_PATH", "users.jsonl"),
		SubscriptionsPath: envOr("SCANNER_SUBSCRIPTIONS_PATH", "subscriptions.jsonl"),
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
