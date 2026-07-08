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
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"cetus-marketdata-scanner/internal/authjwt"
	"cetus-marketdata-scanner/internal/config"
	"cetus-marketdata-scanner/internal/dashboard"
	"cetus-marketdata-scanner/internal/digest"
	"cetus-marketdata-scanner/internal/predicate"
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

// --- session store (in-memory; opaque random tokens) ---

const sessionCookie = "cetus_session"

type sessionStore struct {
	mu sync.Mutex
	m  map[string]sessionEntry
}

type sessionEntry struct {
	uid string
	exp time.Time
}

func newSessionStore() *sessionStore { return &sessionStore{m: map[string]sessionEntry{}} }

func (s *sessionStore) create(uid string, ttl time.Duration) string {
	b := make([]byte, 32)
	rand.Read(b)
	tok := hex.EncodeToString(b)
	s.mu.Lock()
	s.m[tok] = sessionEntry{uid, time.Now().Add(ttl)}
	s.mu.Unlock()
	return tok
}

func (s *sessionStore) get(tok string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[tok]
	if !ok || time.Now().After(e.exp) {
		if ok {
			delete(s.m, tok)
		}
		return "", false
	}
	return e.uid, true
}

func (s *sessionStore) delete(tok string) {
	s.mu.Lock()
	delete(s.m, tok)
	s.mu.Unlock()
}

const (
	authModeLogin = "login" // built-in login + sessions (standalone/dev)
	authModeProxy = "proxy" // trust an identity header from a reverse proxy (caddy-security)
)

// studyQuota returns how many studies a user may own (0 = unlimited). Admins and
// pro users are unlimited; free users are capped by config.
func studyQuota(u user.User, cfg config.Config) int {
	if u.IsAdmin() || u.Tier == user.TierPro {
		return 0
	}
	return cfg.FreeStudyQuota
}

// atoiOr parses s as an int, returning def on failure.
func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}

// splitCSV parses a comma-separated list into trimmed, non-empty items.
func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// isLoopback reports whether a listen address binds only the loopback interface.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	switch host {
	case "", "0.0.0.0", "::":
		return false
	case "localhost":
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

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
	case "users":
		runUsers(log, cfg)
	default:
		log.Error("unknown subcommand", "arg", sub, "want", "scan|digest|serve|anomalies|studies|users")
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
	st, err := store.OpenReadOnly(ctx, cfg.DBPath)
	if err != nil {
		log.Error("open warehouse failed", "db", cfg.DBPath, "error", err)
		os.Exit(1)
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

	snap, err := snapshot.Open(cfg.StoreDB)
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
func runStudies(ctx context.Context, log *slog.Logger, cfg config.Config) {
	st, universe := openUniverse(ctx, log, cfg)
	defer st.Close()

	all, err := study.LoadFile(cfg.StudiesPath)
	if err != nil {
		log.Error("load studies failed", "path", cfg.StudiesPath, "error", err)
		os.Exit(1)
	}
	// Acting user resolved from the registry; tier/role gate which studies run.
	u := actingUser(log, cfg)
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

// runServe serves the login + user dashboard (/) + admin console (/admin) with
// per-session auth. The expensive scan is shared/cached; each request renders the
// logged-in user's tier-accessible studies. SCANNER_SERVE_ADDR sets the address.
func runServe(ctx context.Context, log *slog.Logger, cfg config.Config) {
	st, err := store.OpenReadOnly(ctx, cfg.DBPath)
	if err != nil {
		log.Error("open warehouse failed", "db", cfg.DBPath, "error", err)
		os.Exit(1)
	}
	defer st.Close()

	// The scanner's OWN store (never cetus) — kept open for the server's lifetime.
	snap, err := snapshot.Open(cfg.StoreDB)
	if err != nil {
		log.Error("open snapshot store failed", "store", cfg.StoreDB, "error", err)
		os.Exit(1)
	}
	defer snap.Close()

	users, err := user.OpenStore(cfg.UsersPath)
	if err != nil {
		log.Error("open users failed", "path", cfg.UsersPath, "error", err)
		os.Exit(1)
	}
	studyStore, err := study.OpenStore(cfg.StudiesPath)
	if err != nil {
		log.Error("open studies failed", "path", cfg.StudiesPath, "error", err)
		os.Exit(1)
	}
	if cfg.AuthMode != authModeLogin && cfg.AuthMode != authModeProxy {
		log.Error("bad SCANNER_AUTH_MODE", "mode", cfg.AuthMode, "want", "login|proxy")
		os.Exit(2)
	}
	// Optional JWT verification for proxy mode (verified token beats a raw header).
	var jwtVer interface {
		Verify(string) (string, error)
	}
	if cfg.AuthMode == authModeProxy {
		switch {
		case cfg.JWTJWKSURL != "":
			jwtVer = authjwt.NewJWKS(cfg.JWTJWKSURL, cfg.JWTUserClaim, cfg.JWTIssuer, cfg.JWTAudience)
			log.Info("proxy auth: JWT JWKS verification enabled (rotating RSA keys)", "jwks", cfg.JWTJWKSURL, "header", cfg.JWTHeader, "claim", cfg.JWTUserClaim)
		case cfg.JWTHMACSecret != "":
			jwtVer = authjwt.NewHMAC([]byte(cfg.JWTHMACSecret), cfg.JWTUserClaim, cfg.JWTIssuer, cfg.JWTAudience)
			log.Info("proxy auth: JWT HMAC verification enabled", "header", cfg.JWTHeader, "claim", cfg.JWTUserClaim)
		case cfg.JWTPubKeyFile != "":
			pub, err := authjwt.LoadRSAPublicKeyPEM(cfg.JWTPubKeyFile)
			if err != nil {
				log.Error("load JWT public key failed", "file", cfg.JWTPubKeyFile, "error", err)
				os.Exit(1)
			}
			jwtVer = authjwt.NewRSA(pub, cfg.JWTUserClaim, cfg.JWTIssuer, cfg.JWTAudience)
			log.Info("proxy auth: JWT RSA verification enabled", "header", cfg.JWTHeader, "claim", cfg.JWTUserClaim, "key", cfg.JWTPubKeyFile)
		default:
			log.Warn("proxy auth: no JWT key set — trusting the raw identity header (weaker). Set SCANNER_JWT_HMAC_SECRET or SCANNER_JWT_PUBKEY_FILE to verify a signed token.")
		}
		if !isLoopback(cfg.ServeAddr) && jwtVer == nil {
			log.Warn("proxy auth on a non-loopback bind with no JWT verification: the identity header is spoofable — bind 127.0.0.1 or configure a JWT key",
				"addr", cfg.ServeAddr, "header", cfg.TrustedUserHeader)
		}
	}

	sessions := newSessionStore()
	sessTTL := time.Duration(cfg.SessionHours) * time.Hour
	ttl := time.Duration(cfg.ServeTTLSecs) * time.Second

	// Shared, user-independent scan cache (guarded by mu). Per-user studies run
	// against the shared snapshot at render time.
	var (
		mu      sync.Mutex
		cAt     time.Time
		cRows   []screen.SnapshotRow
		cStats  store.OpsStats
		cFlags  []sentinel.Flag
		cSus    int
		cWat    int
		cDay    time.Time
		cMillis int64
		cSize   int64
	)
	refresh := func() error { // caller holds mu
		start := time.Now()
		universe, err := resolveUniverse(ctx, log, st, cfg)
		if err != nil {
			return err
		}
		if cfg.MaxSymbols > 0 && len(universe) > cfg.MaxSymbols {
			universe = universe[:cfg.MaxSymbols]
		}
		since := time.Now().UTC().AddDate(0, 0, -cfg.DigestLookbackDays).Unix()
		res := scan.Universe(ctx, st, universe, scan.Options{Since: since, MinDollarVol: 0, Workers: cfg.DigestWorkers}, log)
		if err := snap.Load(res.Rows, res.Day.Unix()); err != nil {
			return err
		}
		cRows, cDay = res.Rows, res.Day
		cFlags = sentinel.Tier0(res.Rows, sentinel.DefaultTier0())
		cSus, cWat = sentinel.Counts(cFlags)
		if s2, e := st.Stats(ctx); e == nil {
			cStats = s2
		} else {
			return e
		}
		cSize = 0
		if fi, e := os.Stat(cfg.DBPath); e == nil {
			cSize = fi.Size()
		}
		cMillis = time.Since(start).Milliseconds()
		cAt = time.Now()
		log.Info("scan refreshed", "scanned", res.Scanned, "flagged", len(cFlags), "day", cDay.Format("2006-01-02"))
		return nil
	}

	// modelFor builds a per-user model against the shared cache (refresh if stale).
	modelFor := func(u user.User, force bool) (*dashboard.Model, error) {
		mu.Lock()
		defer mu.Unlock()
		if cRows == nil || time.Since(cAt) >= ttl || force {
			if err := refresh(); err != nil {
				return nil, err
			}
		}
		allStudies := studyStore.All()
		d, err := digest.FromStudies(cDay, cRows, snap, study.Accessible(allStudies, u))
		if err != nil {
			return nil, err
		}
		var mine []study.Study
		for _, s := range allStudies {
			if s.Owner == u.ID {
				mine = append(mine, s)
			}
		}
		return &dashboard.Model{
			Acting: u, SessionAuth: cfg.AuthMode == authModeLogin,
			Stats: cStats, DBSizeBytes: cSize, ScanMillis: cMillis,
			Digest: d, Flags: cFlags, Suspect: cSus, Watch: cWat,
			Users: users.All(), Studies: allStudies, MyStudies: mine,
			StudyQuota: studyQuota(u, cfg),
		}, nil
	}

	// preview runs an ad-hoc study against the current snapshot (the editor's Test
	// button). Shares the scan cache/lock and validates the WHERE by executing it.
	preview := func(st study.Study) ([]snapshot.Match, error) {
		mu.Lock()
		defer mu.Unlock()
		if cRows == nil || time.Since(cAt) >= ttl {
			if err := refresh(); err != nil {
				return nil, err
			}
		}
		return snap.Run(st)
	}

	sessionUser := func(r *http.Request) (user.User, bool) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			return user.User{}, false
		}
		uid, ok := sessions.get(c.Value)
		if !ok {
			return user.User{}, false
		}
		return users.Find(uid)
	}

	// identify resolves the acting user: from the session cookie (login mode) or from
	// the reverse proxy's trusted identity header (proxy mode). A proxy-vouched user
	// with no local profile gets default free/user entitlement.
	identify := func(r *http.Request) (user.User, bool) {
		if cfg.AuthMode == authModeProxy {
			var id string
			if jwtVer != nil { // verify a signed token
				tok := strings.TrimSpace(strings.TrimPrefix(r.Header.Get(cfg.JWTHeader), "Bearer "))
				if tok == "" {
					return user.User{}, false
				}
				uid, err := jwtVer.Verify(tok)
				if err != nil {
					log.Warn("jwt verify failed", "error", err)
					return user.User{}, false
				}
				id = uid
			} else { // trust the raw header (must be loopback-only)
				id = strings.TrimSpace(r.Header.Get(cfg.TrustedUserHeader))
			}
			if id == "" {
				return user.User{}, false
			}
			if u, ok := users.Find(id); ok {
				return u, true
			}
			return user.User{ID: id, Name: id, Tier: user.TierFree, Role: user.RoleUser}, true
		}
		return sessionUser(r)
	}

	// requireUser resolves the acting user or writes the right unauth response.
	requireUser := func(w http.ResponseWriter, r *http.Request) (user.User, bool) {
		u, ok := identify(r)
		if ok {
			return u, true
		}
		if cfg.AuthMode == authModeProxy {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, "401 — no identity from proxy (missing %s header)\n", cfg.TrustedUserHeader)
		} else {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		}
		return user.User{}, false
	}

	page := func(w http.ResponseWriter, r *http.Request, adminOnly bool, render func(*dashboard.Model, io.Writer) error) {
		u, ok := requireUser(w, r)
		if !ok {
			return
		}
		if adminOnly && !u.IsAdmin() {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, "403 — admin only (you are %s / %s)\n", u.ID, u.Role)
			return
		}
		m, err := modelFor(u, r.URL.Query().Get("refresh") != "")
		if err != nil {
			log.Error("render dashboard failed", "error", err)
			http.Error(w, "scan failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := render(m, w); err != nil {
			log.Error("render page failed", "error", err)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	// Public PWA assets (no auth) — make the dashboard installable / standalone.
	mux.HandleFunc("/manifest.webmanifest", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		io.WriteString(w, dashboard.Manifest)
	})
	mux.HandleFunc("/icon.svg", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		io.WriteString(w, dashboard.IconSVG)
	})
	mux.HandleFunc("/sw.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		io.WriteString(w, dashboard.ServiceWorker)
	})

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if cfg.AuthMode == authModeProxy { // the proxy owns auth
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if r.Method == http.MethodPost {
			r.ParseForm()
			u, ok := users.Find(r.FormValue("user"))
			if !ok || !u.CheckPassword(r.FormValue("password")) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				dashboard.Login{Error: "Invalid user or password (or the account is disabled).", Users: users.All()}.HTML(w)
				return
			}
			tok := sessions.create(u.ID, sessTTL)
			http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: tok, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: int(sessTTL.Seconds())})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		dashboard.Login{Users: users.All()}.HTML(w)
	})

	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		if cfg.AuthMode == authModeProxy { // sign-out is handled by the proxy
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if c, err := r.Cookie(sessionCookie); err == nil {
			sessions.delete(c.Value)
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})

	// Admin user CRUD (admin-gated).
	mux.HandleFunc("/admin/users", func(w http.ResponseWriter, r *http.Request) {
		u, ok := requireUser(w, r)
		if !ok {
			return
		}
		if !u.IsAdmin() {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintln(w, "403 — admin only")
			return
		}
		if r.Method != http.MethodPost {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
			return
		}
		r.ParseForm()
		id := strings.TrimSpace(r.FormValue("id"))
		var actErr error
		switch r.FormValue("action") {
		case "create":
			nu := user.User{ID: id, Name: r.FormValue("name"), Tier: user.Tier(r.FormValue("tier")), Role: user.Role(r.FormValue("role")), Groups: splitCSV(r.FormValue("groups"))}
			nu.SetPassword(r.FormValue("password"))
			actErr = users.Create(nu)
		case "disable":
			actErr = users.SetDisabled(id, true)
		case "enable":
			actErr = users.SetDisabled(id, false)
		case "set-pro":
			actErr = users.SetTier(id, user.TierPro)
		case "set-free":
			actErr = users.SetTier(id, user.TierFree)
		case "set-admin":
			actErr = users.SetRole(id, user.RoleAdmin)
		case "set-user":
			actErr = users.SetRole(id, user.RoleUser)
		case "set-groups":
			actErr = users.SetGroups(id, splitCSV(r.FormValue("groups")))
		case "delete":
			if id == u.ID {
				actErr = fmt.Errorf("cannot delete yourself")
			} else {
				actErr = users.Delete(id)
			}
		default:
			actErr = fmt.Errorf("unknown action")
		}
		if actErr != nil {
			log.Warn("user admin action failed", "action", r.FormValue("action"), "id", id, "error", actErr)
		} else {
			log.Info("user admin action", "action", r.FormValue("action"), "id", id, "by", u.ID)
		}
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	})

	// Study editor preview: any logged-in user; non-admin clauses are sandbox-checked.
	mux.HandleFunc("/studies/test", func(w http.ResponseWriter, r *http.Request) {
		u, ok := requireUser(w, r)
		if !ok {
			return
		}
		r.ParseForm()
		where, orderBy := r.FormValue("where"), r.FormValue("order_by")
		w.Header().Set("Content-Type", "application/json")
		type resp struct {
			Count  int      `json:"count"`
			Sample []string `json:"sample"`
			Error  string   `json:"error,omitempty"`
		}
		if !u.IsAdmin() {
			if err := study.ValidateClause(where); err != nil {
				json.NewEncoder(w).Encode(resp{Error: err.Error()})
				return
			}
			if err := study.ValidateClause(orderBy); err != nil {
				json.NewEncoder(w).Encode(resp{Error: err.Error()})
				return
			}
		}
		matches, err := preview(study.Study{Where: where, OrderBy: orderBy, Limit: atoiOr(r.FormValue("limit"), 20)})
		if err != nil {
			json.NewEncoder(w).Encode(resp{Error: err.Error()})
			return
		}
		sample := make([]string, 0, 12)
		for i, m := range matches {
			if i >= 12 {
				break
			}
			sample = append(sample, m.Symbol)
		}
		json.NewEncoder(w).Encode(resp{Count: len(matches), Sample: sample})
	})

	// Structured study editor (docs/ELKO_SCANNER_STUDY_EDITOR_MVP_DESIGN.md): the
	// browser gets a harmless catalog of labels + legal combinations and only ever
	// sends opaque IDs. The catalog is static, so marshal it once.
	catalogBytes, _ := json.Marshal(predicate.BuildCatalog())
	mux.HandleFunc("/api/scanner/catalog", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireUser(w, r); !ok {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(catalogBytes)
	})

	// resultLimit is server-owned (never client-supplied): free tier shows 25 with a
	// +1 probe for has_more; pro sees more.
	resultLimit := func(u user.User) int {
		if u.Tier == user.TierPro || u.IsAdmin() {
			return 101
		}
		return 26
	}

	// Compile a structured Definition → deterministic SQL and run it as a live
	// preview. Every ID is re-validated server-side; unknown/hostile IDs are
	// rejected before any SQL is built, so this is safe on untrusted input.
	mux.HandleFunc("/api/studies/compile", func(w http.ResponseWriter, r *http.Request) {
		u, ok := requireUser(w, r)
		if !ok {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		type resp struct {
			Where   string   `json:"where"`
			OrderBy string   `json:"orderBy"`
			Hash    string   `json:"hash,omitempty"`
			Count   int      `json:"count"`
			Sample  []string `json:"sample"`
			Error   string   `json:"error,omitempty"`
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(resp{Error: "POST only"})
			return
		}
		dec := json.NewDecoder(io.LimitReader(r.Body, 16<<10))
		dec.DisallowUnknownFields()
		var def predicate.Definition
		if err := dec.Decode(&def); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(resp{Error: "bad request: " + err.Error()})
			return
		}
		compiled, err := predicate.Compile(def, resultLimit(u))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(resp{Error: err.Error()})
			return
		}
		matches, err := preview(study.Study{Where: compiled.Where, OrderBy: compiled.OrderBy, Limit: resultLimit(u)})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(resp{Error: err.Error()})
			return
		}
		sample := make([]string, 0, 12)
		for i, m := range matches {
			if i >= 12 {
				break
			}
			sample = append(sample, m.Symbol)
		}
		json.NewEncoder(w).Encode(resp{
			Where: compiled.Where, OrderBy: compiled.OrderBy, Hash: compiled.Hash,
			Count: len(matches), Sample: sample,
		})
	})

	// Study editor save/delete. Admins manage any study; regular users manage their
	// own only — owner forced to self, tier forced free, public reserved for admins,
	// clauses sandboxed, and creation capped by the tier quota.
	// applyStudy validates + persists a study on behalf of u. Non-admin guardrails:
	// owner forced to self, tier forced free, public reserved for admins, group needs
	// membership, clauses sandboxed, new studies capped by the tier quota. Shared by
	// save and import.
	applyStudy := func(u user.User, st study.Study) error {
		st.Key = strings.TrimSpace(st.Key)
		if st.Key == "" {
			return fmt.Errorf("key required")
		}
		existing, exists := studyStore.Get(st.Key)
		if exists && !u.IsAdmin() && existing.Owner != u.ID {
			return fmt.Errorf("study %q is not yours", st.Key)
		}
		if !u.IsAdmin() {
			st.Owner = u.ID
			st.Tier = user.TierFree
			switch st.Visibility {
			case study.VisGroup:
				if !u.InGroup(st.Group) {
					return fmt.Errorf("not a member of group %q", st.Group)
				}
			default: // public/private/unset → non-admins can't publish public; coerce to private
				st.Visibility = study.VisPrivate
			}
			if err := study.ValidateClause(st.Where); err != nil {
				return fmt.Errorf("WHERE: %w", err)
			}
			if err := study.ValidateClause(st.OrderBy); err != nil {
				return fmt.Errorf("ORDER BY: %w", err)
			}
			if !exists {
				if q := studyQuota(u, cfg); q > 0 {
					owned := 0
					for _, s := range studyStore.All() {
						if s.Owner == u.ID {
							owned++
						}
					}
					if owned >= q {
						return fmt.Errorf("study limit reached (%d) on the %s tier — upgrade for more", q, u.Tier)
					}
				}
			}
		}
		if strings.TrimSpace(st.Where) != "" { // catch SQL errors before persisting
			if _, perr := preview(study.Study{Where: st.Where, OrderBy: st.OrderBy, Limit: 1}); perr != nil {
				return fmt.Errorf("invalid WHERE: %w", perr)
			}
		}
		return studyStore.Upsert(st)
	}

	mux.HandleFunc("/studies", func(w http.ResponseWriter, r *http.Request) {
		u, ok := requireUser(w, r)
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		r.ParseForm()
		back := "/"
		if u.IsAdmin() {
			back = "/admin"
		}
		key := strings.TrimSpace(r.FormValue("key"))
		switch r.FormValue("action") {
		case "save":
			st := study.Study{
				Key: key, Title: r.FormValue("title"), Emoji: r.FormValue("emoji"),
				Owner: r.FormValue("owner"), Visibility: study.Visibility(r.FormValue("visibility")),
				Group: r.FormValue("group"), Tier: user.Tier(r.FormValue("tier")),
				Where: r.FormValue("where"), OrderBy: r.FormValue("order_by"), Limit: atoiOr(r.FormValue("limit"), 0),
			}
			if err := applyStudy(u, st); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, "cannot save study: %v", err)
				return
			}
			log.Info("study saved", "key", st.Key, "by", u.ID)
		case "delete":
			existing, exists := studyStore.Get(key)
			if exists && !u.IsAdmin() && existing.Owner != u.ID {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprintln(w, "403 — not your study")
				return
			}
			if err := studyStore.Delete(key); err != nil {
				log.Warn("study delete failed", "key", key, "error", err)
			} else {
				log.Info("study deleted", "key", key, "by", u.ID)
			}
		}
		http.Redirect(w, r, back, http.StatusSeeOther)
	})

	// Export the acting user's studies (all, for admins) as JSONL — download.
	mux.HandleFunc("/studies/export", func(w http.ResponseWriter, r *http.Request) {
		u, ok := requireUser(w, r)
		if !ok {
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Content-Disposition", `attachment; filename="studies.jsonl"`)
		enc := json.NewEncoder(w)
		for _, s := range studyStore.All() {
			if u.IsAdmin() || s.Owner == u.ID {
				_ = enc.Encode(s)
			}
		}
	})

	// Import studies from pasted JSONL — each applied through the same guardrails.
	mux.HandleFunc("/studies/import", func(w http.ResponseWriter, r *http.Request) {
		u, ok := requireUser(w, r)
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		r.ParseForm()
		studies, err := study.LoadJSONL(strings.NewReader(r.FormValue("jsonl")))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "parse error: %v", err)
			return
		}
		imported, failed, firstErr := 0, 0, ""
		for _, st := range studies {
			if err := applyStudy(u, st); err != nil {
				failed++
				if firstErr == "" {
					firstErr = err.Error()
				}
			} else {
				imported++
			}
		}
		log.Info("studies imported", "imported", imported, "failed", failed, "by", u.ID)
		if imported == 0 && failed > 0 {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "imported 0 of %d — first error: %s", failed, firstErr)
			return
		}
		back := "/"
		if u.IsAdmin() {
			back = "/admin"
		}
		http.Redirect(w, r, back, http.StatusSeeOther)
	})

	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		page(w, r, true, func(m *dashboard.Model, out io.Writer) error { return m.AdminHTML(out) })
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		page(w, r, false, func(m *dashboard.Model, out io.Writer) error { return m.IndexHTML(out) })
	})

	// Bind first, so a port collision fails immediately with a clear error instead of
	// after a misleading "serving" log.
	ln, err := net.Listen("tcp", cfg.ServeAddr)
	if err != nil {
		log.Error("cannot bind dashboard address (port already in use?)", "addr", cfg.ServeAddr, "error", err)
		os.Exit(1)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shCtx)
	}()

	log.Info("dashboard serving", "addr", ln.Addr().String(), "db", cfg.DBPath, "ttl_secs", cfg.ServeTTLSecs)
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Error("serve failed", "error", err)
		os.Exit(1)
	}
	log.Info("dashboard stopped")
}
