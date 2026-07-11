// Package serve implements the live HTTP dashboard: login, user dashboard,
// admin console, study editor, and the REST API.
package serve

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"cetus-marketdata-scanner/internal/admin"
	"cetus-marketdata-scanner/internal/alert"
	"cetus-marketdata-scanner/internal/api"
	"cetus-marketdata-scanner/internal/authjwt"
	"cetus-marketdata-scanner/internal/backtest"
	"cetus-marketdata-scanner/internal/bootstrap"
	"cetus-marketdata-scanner/internal/config"
	"cetus-marketdata-scanner/internal/dashboard"
	"cetus-marketdata-scanner/internal/digest"
	"cetus-marketdata-scanner/internal/groups"
	"cetus-marketdata-scanner/internal/permissions"
	"cetus-marketdata-scanner/internal/predicate"
	"cetus-marketdata-scanner/internal/results"
	"cetus-marketdata-scanner/internal/roles"
	"cetus-marketdata-scanner/internal/scan"
	"cetus-marketdata-scanner/internal/screen"
	"cetus-marketdata-scanner/internal/sentinel"
	"cetus-marketdata-scanner/internal/snapshot"
	"cetus-marketdata-scanner/internal/store"
	"cetus-marketdata-scanner/internal/study"
	"cetus-marketdata-scanner/internal/throttle"
	"cetus-marketdata-scanner/internal/user"
)

const (
	AuthModeLogin = "login"
	AuthModeProxy = "proxy"

	sessionCookie = "cetus_session"
)

// Server is the live dashboard HTTP server.
type Server struct {
	cfg config.Config
	log *slog.Logger
	ctx context.Context

	warehouse *store.Store
	snap      *snapshot.DB
	users     *user.Store
	studies   *study.Store
	subs      *study.SubscriptionStore
	groups    *groups.Store
	results   *results.Store
	roles     *roles.Store
	throttler *throttle.Throttler
	permCheck *permissions.Checker
	admin     *admin.Handler

	jwtVer interface {
		Verify(string) (string, error)
	}
	signer *authjwt.Signer

	sessions *sessionStore
	sessTTL  time.Duration
	ttl      time.Duration

	// Shared, user-independent scan cache.
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

	catalogBytes []byte
}

// New creates a Server. The caller must invoke Run to start serving.
func New(ctx context.Context, log *slog.Logger, cfg config.Config) (*Server, error) {
	st, err := store.OpenReadOnly(ctx, cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open warehouse: %w", err)
	}

	snap, err := snapshot.Open(cfg.StoreDB)
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("open snapshot store: %w", err)
	}

	users, err := user.OpenStore(cfg.UsersPath)
	if err != nil {
		st.Close()
		snap.Close()
		return nil, fmt.Errorf("open users: %w", err)
	}

	studyStore, err := study.OpenStore(cfg.StudiesPath)
	if err != nil {
		st.Close()
		snap.Close()
		return nil, fmt.Errorf("open studies: %w", err)
	}

	subStore, err := study.OpenSubscriptionStore(cfg.SubscriptionsPath)
	if err != nil {
		st.Close()
		snap.Close()
		return nil, fmt.Errorf("open subscriptions: %w", err)
	}

	if cfg.AuthMode != AuthModeLogin && cfg.AuthMode != AuthModeProxy {
		st.Close()
		snap.Close()
		return nil, fmt.Errorf("bad SCANNER_AUTH_MODE %q (want login|proxy)", cfg.AuthMode)
	}

	var jwtVer interface {
		Verify(string) (string, error)
	}
	if cfg.AuthMode == AuthModeProxy {
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
				st.Close()
				snap.Close()
				return nil, fmt.Errorf("load JWT public key: %w", err)
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

	var apiSigner *authjwt.Signer
	if cfg.JWTSignSecret != "" {
		apiSigner = authjwt.NewHMACSigner([]byte(cfg.JWTSignSecret), cfg.JWTIssuer, time.Duration(cfg.JWTSignTTLHours)*time.Hour)
		log.Info("JWT signing enabled for API login endpoint")
	}

	catalogBytes, _ := json.Marshal(predicate.BuildCatalog())

	// Initialize groups and results stores
	groupsStore := groups.NewStore(snap.DB())
	resultsStore := results.NewStore(snap.DB())
	permChecker := permissions.NewChecker(permissions.NewDBAccessChecker(snap.DB()))

	// Initialize roles and throttling
	rolesStore := roles.NewStore(snap.DB())
	if err := rolesStore.Init(); err != nil {
		return nil, fmt.Errorf("init roles: %w", err)
	}
	if err := rolesStore.Bootstrap("roles.json"); err != nil {
		return nil, fmt.Errorf("bootstrap roles: %w", err)
	}

	// Bootstrap default admin user if no users exist
	if err := bootstrap.Bootstrap(snap.DB(), users, rolesStore, log); err != nil {
		return nil, fmt.Errorf("bootstrap users: %w", err)
	}

	throttler := throttle.NewThrottler(snap.DB(), rolesStore)
	if err := throttler.Init(); err != nil {
		return nil, fmt.Errorf("init throttler: %w", err)
	}

	// Initialize admin panel
	adminHandler, err := admin.NewHandler(snap.DB(), users, rolesStore, throttler, log)
	if err != nil {
		return nil, fmt.Errorf("init admin: %w", err)
	}

	return &Server{
		cfg: cfg, log: log, ctx: ctx,
		warehouse: st, snap: snap, users: users, studies: studyStore, subs: subStore,
		groups: groupsStore, results: resultsStore, permCheck: permChecker,
		roles: rolesStore, throttler: throttler, admin: adminHandler,
		jwtVer: jwtVer, signer: apiSigner,
		sessions:     newSessionStore(),
		sessTTL:      time.Duration(cfg.SessionHours) * time.Hour,
		ttl:          time.Duration(cfg.ServeTTLSecs) * time.Second,
		catalogBytes: catalogBytes,
	}, nil
}

// Run starts the HTTP server and blocks until ctx is cancelled or the server stops.
func (s *Server) Run() {
	defer s.warehouse.Close()
	defer s.snap.Close()

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	ln, err := net.Listen("tcp", s.cfg.ServeAddr)
	if err != nil {
		s.log.Error("cannot bind dashboard address (port already in use?)", "addr", s.cfg.ServeAddr, "error", err)
		os.Exit(1)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-s.ctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shCtx)
	}()

	s.log.Info("dashboard serving", "addr", ln.Addr().String(), "db", s.cfg.DBPath, "ttl_secs", s.cfg.ServeTTLSecs)
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		s.log.Error("serve failed", "error", err)
		os.Exit(1)
	}
	s.log.Info("dashboard stopped")
}

// --- scan cache ---

func (s *Server) refresh() error {
	start := time.Now()
	universe, err := s.resolveUniverse()
	if err != nil {
		return err
	}
	if s.cfg.MaxSymbols > 0 && len(universe) > s.cfg.MaxSymbols {
		universe = universe[:s.cfg.MaxSymbols]
	}
	since := time.Now().UTC().AddDate(0, 0, -s.cfg.DigestLookbackDays).Unix()
	res := scan.Universe(s.ctx, s.warehouse, universe, scan.Options{Since: since, MinDollarVol: 0, Workers: s.cfg.DigestWorkers}, s.log)
	if err := s.snap.Load(res.Rows, res.Day.Unix()); err != nil {
		return err
	}
	s.cRows, s.cDay = res.Rows, res.Day
	s.cFlags = sentinel.Tier0(res.Rows, sentinel.DefaultTier0())
	s.cSus, s.cWat = sentinel.Counts(s.cFlags)
	if s2, e := s.warehouse.Stats(s.ctx); e == nil {
		s.cStats = s2
	} else {
		return e
	}
	s.cSize = 0
	if fi, e := os.Stat(s.cfg.DBPath); e == nil {
		s.cSize = fi.Size()
	}
	s.cMillis = time.Since(start).Milliseconds()
	s.cAt = time.Now()
	s.log.Info("scan refreshed", "scanned", res.Scanned, "flagged", len(s.cFlags), "day", s.cDay.Format("2006-01-02"))
	return nil
}

func (s *Server) modelFor(u user.User, force bool) (*dashboard.Model, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cRows == nil || time.Since(s.cAt) >= s.ttl || force {
		if err := s.refresh(); err != nil {
			return nil, err
		}
	}
	allStudies := s.studies.All()
	d, err := digest.FromStudies(s.cDay, s.cRows, s.snap, study.Accessible(allStudies, u))
	if err != nil {
		return nil, err
	}
	var mine []study.Study
	for _, st := range allStudies {
		if st.Owner == u.ID {
			mine = append(mine, st)
		}
	}
	return &dashboard.Model{
		Acting: u, SessionAuth: s.cfg.AuthMode == AuthModeLogin,
		Stats: s.cStats, DBSizeBytes: s.cSize, ScanMillis: s.cMillis,
		Digest: d, Flags: s.cFlags, Suspect: s.cSus, Watch: s.cWat,
		Users: s.users.All(), Studies: allStudies, MyStudies: mine,
		StudyQuota: studyQuota(u, s.cfg),
	}, nil
}

func (s *Server) preview(st study.Study) ([]snapshot.Match, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cRows == nil || time.Since(s.cAt) >= s.ttl {
		if err := s.refresh(); err != nil {
			return nil, err
		}
	}
	return s.snap.Run(st)
}

// --- auth ---

func (s *Server) sessionUser(r *http.Request) (user.User, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return user.User{}, false
	}
	uid, ok := s.sessions.get(c.Value)
	if !ok {
		return user.User{}, false
	}
	return s.users.Find(uid)
}

func (s *Server) identify(r *http.Request) (user.User, bool) {
	if s.cfg.AuthMode == AuthModeProxy {
		var id string
		if s.jwtVer != nil {
			tok := strings.TrimSpace(strings.TrimPrefix(r.Header.Get(s.cfg.JWTHeader), "Bearer "))
			if tok == "" {
				return user.User{}, false
			}
			uid, err := s.jwtVer.Verify(tok)
			if err != nil {
				s.log.Warn("jwt verify failed", "error", err)
				return user.User{}, false
			}
			id = uid
		} else {
			id = strings.TrimSpace(r.Header.Get(s.cfg.TrustedUserHeader))
		}
		if id == "" {
			return user.User{}, false
		}
		if u, ok := s.users.Find(id); ok {
			return u, true
		}
		return user.User{ID: id, Name: id, Tier: user.TierFree, Role: user.RoleUser}, true
	}
	return s.sessionUser(r)
}

func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (user.User, bool) {
	u, ok := s.identify(r)
	if ok {
		return u, true
	}
	if s.cfg.AuthMode == AuthModeProxy {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, "401 — no identity from proxy (missing %s header)\n", s.cfg.TrustedUserHeader)
	} else {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
	return user.User{}, false
}

func (s *Server) page(w http.ResponseWriter, r *http.Request, adminOnly bool, render func(*dashboard.Model, io.Writer) error) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if adminOnly && !u.IsAdmin() {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "403 — admin only (you are %s / %s)\n", u.ID, u.Role)
		return
	}
	m, err := s.modelFor(u, r.URL.Query().Get("refresh") != "")
	if err != nil {
		s.log.Error("render dashboard failed", "error", err)
		http.Error(w, "scan failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := render(m, w); err != nil {
		s.log.Error("render page failed", "error", err)
	}
}

// --- universe resolution ---

func (s *Server) resolveUniverse() ([]string, error) {
	spec := strings.TrimSpace(s.cfg.Universe)
	switch {
	case spec == "" || spec == "all":
		return s.warehouse.Universe(s.ctx)
	case spec == "common":
		return s.warehouse.UniverseCommon(s.ctx)
	case strings.HasPrefix(spec, "exchange:"):
		return s.warehouse.UniverseExchange(s.ctx, strings.TrimPrefix(spec, "exchange:"))
	case strings.HasPrefix(spec, "index:"), strings.HasPrefix(spec, "list:"):
		code := strings.TrimPrefix(strings.TrimPrefix(spec, "index:"), "list:")
		syms, err := s.warehouse.UniverseIndex(s.ctx, code)
		if err != nil {
			return nil, err
		}
		if len(syms) == 0 {
			s.log.Warn("index not seeded — falling back to common stock", "index", code)
			return s.warehouse.UniverseCommon(s.ctx)
		}
		return syms, nil
	case strings.HasPrefix(spec, "file:"):
		return s.universeFromFile(strings.TrimPrefix(spec, "file:"))
	default:
		return nil, fmt.Errorf("bad SCANNER_UNIVERSE %q (want all|common|index:X|exchange:X|file:PATH)", spec)
	}
}

func (s *Server) universeFromFile(path string) ([]string, error) {
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
	all, err := s.warehouse.Universe(s.ctx)
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

// --- helpers ---

func studyQuota(u user.User, cfg config.Config) int {
	if u.IsAdmin() || u.Tier == user.TierPro {
		return 0
	}
	return cfg.FreeStudyQuota
}

func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

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

// --- session store ---

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

// resultLimit is server-owned (never client-supplied): free tier shows 25 with a
// +1 probe for has_more; pro sees more.
func resultLimit(u user.User) int {
	if u.Tier == user.TierPro || u.IsAdmin() {
		return 101
	}
	return 26
}

// ensure interfaces are satisfied
var _ = alert.NewDetector
var _ = backtest.NewEngine
var _ = api.NewHandlerFull
