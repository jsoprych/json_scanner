package serve

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"cetus-marketdata-scanner/internal/alert"
	"cetus-marketdata-scanner/internal/api"
	"cetus-marketdata-scanner/internal/backtest"
	"cetus-marketdata-scanner/internal/dashboard"
	"cetus-marketdata-scanner/internal/predicate"
	"cetus-marketdata-scanner/internal/study"
	"cetus-marketdata-scanner/internal/user"
)

// safeMux wraps http.ServeMux to track registered routes and prevent conflicts
type safeMux struct {
	*http.ServeMux
	routes map[string]bool
}

func newSafeMux() *safeMux {
	return &safeMux{
		ServeMux: http.NewServeMux(),
		routes:   make(map[string]bool),
	}
}

func (m *safeMux) HandleFunc(pattern string, handler http.HandlerFunc) {
	if m.routes[pattern] {
		panic(fmt.Sprintf("route conflict: %q already registered", pattern))
	}
	m.routes[pattern] = true
	m.ServeMux.HandleFunc(pattern, handler)
}

func (m *safeMux) Handle(pattern string, handler http.Handler) {
	if m.routes[pattern] {
		panic(fmt.Sprintf("route conflict: %q already registered", pattern))
	}
	m.routes[pattern] = true
	m.ServeMux.Handle(pattern, handler)
}

func (s *Server) registerRoutes(mux *safeMux) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

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

	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/logout", s.handleLogout)
	mux.HandleFunc("/studies/test", s.handleStudiesTest)
	mux.HandleFunc("/api/scanner/catalog", s.handleCatalog)
	mux.HandleFunc("/api/studies/compile", s.handleCompile)
	mux.HandleFunc("/studies", s.handleStudies)
	mux.HandleFunc("/studies/export", s.handleStudiesExport)
	mux.HandleFunc("/studies/import", s.handleStudiesImport)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		s.page(w, r, false, func(m *dashboard.Model, out io.Writer) error { return m.IndexHTML(out) })
	})

	detector := alert.NewDetector(s.snap)
	backtestEngine := backtest.NewEngine(s.snap)
	apiHandler := api.NewHandlerFull(
		s.snap, s.studies, s.warehouse, s.users, s.subs,
		s.groups, s.results, s.roles, s.permCheck, s.throttler,
		detector, backtestEngine, s.signer, s.jwtVer, s.log,
	)
	mux.Handle("/api/v1/", apiHandler.Router())

	// Register admin panel routes
	if s.admin != nil {
		s.admin.RegisterRoutes(mux.ServeMux)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AuthMode == AuthModeProxy {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodPost {
		r.ParseForm()
		u, ok := s.users.Find(r.FormValue("user"))
		if !ok || !u.CheckPassword(r.FormValue("password")) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			dashboard.Login{Error: "Invalid user or password (or the account is disabled).", Users: s.users.All()}.HTML(w)
			return
		}
		tok := s.sessions.create(u.ID, s.sessTTL)
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: tok, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: int(s.sessTTL.Seconds())})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	dashboard.Login{Users: s.users.All()}.HTML(w)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AuthMode == AuthModeProxy {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.sessions.delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
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
		actErr = s.users.Create(nu)
	case "disable":
		actErr = s.users.SetDisabled(id, true)
	case "enable":
		actErr = s.users.SetDisabled(id, false)
	case "set-pro":
		actErr = s.users.SetTier(id, user.TierPro)
	case "set-free":
		actErr = s.users.SetTier(id, user.TierFree)
	case "set-admin":
		actErr = s.users.SetRole(id, user.RoleAdmin)
	case "set-user":
		actErr = s.users.SetRole(id, user.RoleUser)
	case "set-groups":
		actErr = s.users.SetGroups(id, splitCSV(r.FormValue("groups")))
	case "delete":
		if id == u.ID {
			actErr = fmt.Errorf("cannot delete yourself")
		} else {
			actErr = s.users.Delete(id)
		}
	default:
		actErr = fmt.Errorf("unknown action")
	}
	if actErr != nil {
		s.log.Warn("user admin action failed", "action", r.FormValue("action"), "id", id, "error", actErr)
	} else {
		s.log.Info("user admin action", "action", r.FormValue("action"), "id", id, "by", u.ID)
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) handleStudiesTest(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
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
	matches, err := s.preview(study.Study{Where: where, OrderBy: orderBy, Limit: atoiOr(r.FormValue("limit"), 20)})
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
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(s.catalogBytes)
}

func (s *Server) handleCompile(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
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
	matches, err := s.preview(study.Study{Where: compiled.Where, OrderBy: compiled.OrderBy, Limit: resultLimit(u)})
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
}

func (s *Server) applyStudy(u user.User, st study.Study) error {
	st.Key = strings.TrimSpace(st.Key)
	if st.Key == "" {
		return fmt.Errorf("key required")
	}
	existing, exists := s.studies.Get(st.Key)
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
		default:
			st.Visibility = study.VisPrivate
		}
		if err := study.ValidateClause(st.Where); err != nil {
			return fmt.Errorf("WHERE: %w", err)
		}
		if err := study.ValidateClause(st.OrderBy); err != nil {
			return fmt.Errorf("ORDER BY: %w", err)
		}
		if !exists {
			if q := studyQuota(u, s.cfg); q > 0 {
				owned := 0
				for _, st := range s.studies.All() {
					if st.Owner == u.ID {
						owned++
					}
				}
				if owned >= q {
					return fmt.Errorf("study limit reached (%d) on the %s tier — upgrade for more", q, u.Tier)
				}
			}
		}
	}
	if strings.TrimSpace(st.Where) != "" {
		if _, perr := s.preview(study.Study{Where: st.Where, OrderBy: st.OrderBy, Limit: 1}); perr != nil {
			return fmt.Errorf("invalid WHERE: %w", perr)
		}
	}
	return s.studies.Upsert(st)
}

func (s *Server) handleStudies(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
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
		if err := s.applyStudy(u, st); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "cannot save study: %v", err)
			return
		}
		s.log.Info("study saved", "key", st.Key, "by", u.ID)
	case "delete":
		existing, exists := s.studies.Get(key)
		if exists && !u.IsAdmin() && existing.Owner != u.ID {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintln(w, "403 — not your study")
			return
		}
		if err := s.studies.Delete(key); err != nil {
			s.log.Warn("study delete failed", "key", key, "error", err)
		} else {
			s.log.Info("study deleted", "key", key, "by", u.ID)
		}
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (s *Server) handleStudiesExport(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", `attachment; filename="studies.jsonl"`)
	enc := json.NewEncoder(w)
	for _, st := range s.studies.All() {
		if u.IsAdmin() || st.Owner == u.ID {
			_ = enc.Encode(st)
		}
	}
}

func (s *Server) handleStudiesImport(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
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
		if err := s.applyStudy(u, st); err != nil {
			failed++
			if firstErr == "" {
				firstErr = err.Error()
			}
		} else {
			imported++
		}
	}
	s.log.Info("studies imported", "imported", imported, "failed", failed, "by", u.ID)
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
}
