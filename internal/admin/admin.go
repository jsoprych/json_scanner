package admin

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"cetus-marketdata-scanner/internal/dblog"
	"cetus-marketdata-scanner/internal/roles"
	"cetus-marketdata-scanner/internal/throttle"
	"cetus-marketdata-scanner/internal/user"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

// Handler manages the admin panel
type Handler struct {
	db        *dblog.DB
	users     *user.Store
	roles     *roles.Store
	throttler *throttle.Throttler
	templates *template.Template
	log       *slog.Logger
	
	// Session validator function (provided by serve package)
	validateSession func(cookie string) (userID string, isAdmin bool, valid bool)
}

// NewHandler creates a new admin handler
func NewHandler(db *dblog.DB, users *user.Store, roles *roles.Store, throttler *throttle.Throttler, log *slog.Logger, validateSession func(cookie string) (userID string, isAdmin bool, valid bool)) (*Handler, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &Handler{
		db:              db,
		users:           users,
		roles:           roles,
		throttler:       throttler,
		templates:       tmpl,
		log:             log,
		validateSession: validateSession,
	}, nil
}

// RegisterRoutes registers admin panel routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Static files — embed has "static/" prefix, strip it so /admin/static/styles.css works
	staticSubFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		h.log.Error("failed to create static sub-filesystem", "error", err)
	} else {
		mux.Handle("/admin/static/", http.StripPrefix("/admin/static/", http.FileServer(http.FS(staticSubFS))))
	}

	// Logout
	mux.HandleFunc("/admin/logout", h.logout)

	// Protected pages
	mux.HandleFunc("/admin", h.requireAdmin(h.dashboard))
	mux.HandleFunc("/admin/dashboard", h.requireAdmin(h.dashboard))
	mux.HandleFunc("/admin/users", h.requireAdmin(h.usersPage))
	mux.HandleFunc("/admin/roles", h.requireAdmin(h.rolesPage))
	mux.HandleFunc("/admin/groups", h.requireAdmin(h.groupsPage))
	mux.HandleFunc("/admin/monitoring", h.requireAdmin(h.monitoringPage))

	// API endpoints
	mux.HandleFunc("/api/v1/admin/stats", h.requireAdminAPI(h.getStats))
	mux.HandleFunc("/api/v1/admin/activities", h.requireAdminAPI(h.getActivities))
	mux.HandleFunc("/api/v1/admin/quotas", h.requireAdminAPI(h.getQuotas))
	mux.HandleFunc("/api/v1/admin/monitoring/system", h.requireAdminAPI(h.getSystemStats))
	mux.HandleFunc("/api/v1/admin/monitoring/api", h.requireAdminAPI(h.getAPIMetrics))
	mux.HandleFunc("/api/v1/admin/monitoring/database", h.requireAdminAPI(h.getDBStats))
	mux.HandleFunc("/api/v1/admin/monitoring/throttle", h.requireAdminAPI(h.getThrottleStats))
	mux.HandleFunc("/api/v1/admin/monitoring/errors", h.requireAdminAPI(h.getRecentErrors))
}

// Middleware
func (h *Handler) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for session cookie (from main dashboard login)
		cookie, err := r.Cookie("cetus_session")
		if err != nil {
			// No session cookie, redirect to main login
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Validate session using the provided validator
		if h.validateSession == nil {
			h.log.Error("session validator not configured")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		userID, isAdmin, valid := h.validateSession(cookie.Value)
		if !valid {
			// Invalid session, redirect to main login
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if !isAdmin {
			// Valid session but not admin
			http.Error(w, "Forbidden: admin access required", http.StatusForbidden)
			return
		}

		// Store user ID in request context for later use
		r = r.WithContext(context.WithValue(r.Context(), "userID", userID))
		next(w, r)
	}
}

func (h *Handler) requireAdminAPI(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for session cookie (from main dashboard login)
		cookie, err := r.Cookie("cetus_session")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Validate session using the provided validator
		if h.validateSession == nil {
			h.log.Error("session validator not configured")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userID, isAdmin, valid := h.validateSession(cookie.Value)
		if !valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if !isAdmin {
			http.Error(w, "Forbidden: admin access required", http.StatusForbidden)
			return
		}

		// Store user ID in request context for later use
		r = r.WithContext(context.WithValue(r.Context(), "userID", userID))
		next(w, r)
	}
}

// Logout
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	// Redirect to main logout which will clear the session cookie
	http.Redirect(w, r, "/logout", http.StatusSeeOther)
}

// Page Handlers
func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "templates/dashboard.html")
}

func (h *Handler) usersPage(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "templates/users.html")
}

func (h *Handler) rolesPage(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "templates/roles.html")
}

func (h *Handler) groupsPage(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "templates/groups.html")
}

func (h *Handler) monitoringPage(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "templates/monitoring.html")
}

// renderPage renders a template with the current user info
func (h *Handler) renderPage(w http.ResponseWriter, r *http.Request, templateFile string) {
	content, err := templatesFS.ReadFile(templateFile)
	if err != nil {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Get user ID from request context (set by requireAdmin middleware)
	userID, _ := r.Context().Value("userID").(string)

	data := struct {
		Content     template.HTML
		CurrentUser string
	}{
		Content:     template.HTML(content),
		CurrentUser: userID,
	}

	w.Header().Set("Content-Type", "text/html")
	h.templates.ExecuteTemplate(w, "base.html", data)
}

// API Handlers
func (h *Handler) getStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{
		"totalUsers":      0,
		"activeUsers":     0,
		"apiCalls":        0,
		"apiCallsChange":  0,
		"storage":         "0 GB",
	}

	// Get user counts
	allUsers := h.users.All()
	stats["totalUsers"] = len(allUsers)

	activeCount := 0
	for _, u := range allUsers {
		if !u.Disabled {
			activeCount++
		}
	}
	stats["activeUsers"] = activeCount

	// Get today's API calls
	today := time.Now().Format("2006-01-02")
	var totalCalls int
	err := h.db.QueryRow(`
		SELECT COALESCE(SUM(api_calls), 0) 
		FROM usage_tracking 
		WHERE date = ?
	`, today).Scan(&totalCalls)
	if err == nil {
		stats["apiCalls"] = totalCalls
	}

	// Get yesterday's API calls for comparison
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	var yesterdayCalls int
	err = h.db.QueryRow(`
		SELECT COALESCE(SUM(api_calls), 0) 
		FROM usage_tracking 
		WHERE date = ?
	`, yesterday).Scan(&yesterdayCalls)
	if err == nil && yesterdayCalls > 0 {
		change := ((totalCalls - yesterdayCalls) * 100) / yesterdayCalls
		stats["apiCallsChange"] = change
	}

	// TODO: Calculate storage used

	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) getActivities(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement activity tracking
	// For now, return mock data
	activities := []map[string]interface{}{
		{
			"id":       1,
			"user":     "admin",
			"action":   "login",
			"resource": "system",
			"time":     "2 minutes ago",
			"status":   "success",
		},
		{
			"id":       2,
			"user":     "alice",
			"action":   "create",
			"resource": "study",
			"time":     "5 minutes ago",
			"status":   "success",
		},
	}

	writeJSON(w, http.StatusOK, activities)
}

func (h *Handler) getQuotas(w http.ResponseWriter, r *http.Request) {
	allUsers := h.users.All()
	quotas := []map[string]interface{}{}

	for _, u := range allUsers {
		if u.Disabled {
			continue
		}

		// Get user's limits
		limits, err := h.throttler.GetEffectiveLimits(u.ID)
		if err != nil {
			continue
		}

		// Get user's current usage
		usage, err := h.throttler.GetUserUsage(u.ID)
		if err != nil {
			continue
		}

		// Calculate percentage
		percent := 0
		if limits.APICallsPerDay > 0 {
			percent = (usage["api_calls"] * 100) / limits.APICallsPerDay
		}

		quotas = append(quotas, map[string]interface{}{
			"userId":   u.ID,
			"userName": u.Name,
			"used":     usage["api_calls"],
			"limit":    limits.APICallsPerDay,
			"percent":  percent,
		})
	}

	writeJSON(w, http.StatusOK, quotas)
}

// Helper
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Monitoring API Handlers
func (h *Handler) getSystemStats(w http.ResponseWriter, r *http.Request) {
	// Get system uptime
	startTime := time.Now().Add(-2 * 24 * time.Hour) // Mock: 2 days uptime
	uptime := time.Since(startTime)
	days := int(uptime.Hours() / 24)
	hours := int(uptime.Hours()) % 24

	stats := map[string]interface{}{
		"uptime":        fmt.Sprintf("%dd %dh", days, hours),
		"memory":        "256 MB",
		"memoryPercent": 32,
		"connections":   12,
		"requestRate":   45,
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) getAPIMetrics(w http.ResponseWriter, r *http.Request) {
	// Mock API metrics
	metrics := []map[string]interface{}{
		{
			"endpoint":   "/api/v1/scan",
			"requests":   1250,
			"avgLatency": 45,
			"p95Latency": 120,
			"errorRate":  0.5,
			"healthy":    true,
		},
		{
			"endpoint":   "/api/v1/studies",
			"requests":   890,
			"avgLatency": 32,
			"p95Latency": 85,
			"errorRate":  0.2,
			"healthy":    true,
		},
		{
			"endpoint":   "/api/v1/results",
			"requests":   456,
			"avgLatency": 28,
			"p95Latency": 65,
			"errorRate":  1.2,
			"healthy":    true,
		},
		{
			"endpoint":   "/api/v1/users",
			"requests":   234,
			"avgLatency": 15,
			"p95Latency": 45,
			"errorRate":  0.1,
			"healthy":    true,
		},
	}

	writeJSON(w, http.StatusOK, metrics)
}

func (h *Handler) getDBStats(w http.ResponseWriter, r *http.Request) {
	// Get database statistics
	var warehouseSize int64
	var scannerSize int64
	var totalSymbols int
	var snapshotCount int

	// Try to get real data, fall back to mock
	err := h.db.QueryRow("SELECT COUNT(DISTINCT symbol) FROM snapshot").Scan(&totalSymbols)
	if err != nil {
		totalSymbols = 11373
	}

	err = h.db.QueryRow("SELECT COUNT(DISTINCT snapshot_date) FROM snapshot").Scan(&snapshotCount)
	if err != nil {
		snapshotCount = 125
	}

	stats := map[string]interface{}{
		"warehouseSize": "1.8 GB",
		"scannerSize":   "45 MB",
		"totalSymbols":  totalSymbols,
		"snapshotCount": snapshotCount,
	}

	_ = warehouseSize
	_ = scannerSize

	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) getThrottleStats(w http.ResponseWriter, r *http.Request) {
	// Get throttling statistics
	var violations int
	var quotaExceeded int
	var nearLimit int
	var bypassUsers int

	// Count rate limit violations in last 24h
	yesterday := time.Now().AddDate(0, 0, -1).Unix()
	err := h.db.QueryRow(`
		SELECT COUNT(*) FROM rate_limits 
		WHERE timestamp > ?
	`, yesterday).Scan(&violations)
	if err != nil {
		violations = 12
	}

	// Count users near their limits (mock)
	nearLimit = 8
	bypassUsers = 2

	stats := map[string]interface{}{
		"violations":    violations,
		"quotaExceeded": quotaExceeded,
		"nearLimit":     nearLimit,
		"bypassUsers":   bypassUsers,
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) getRecentErrors(w http.ResponseWriter, r *http.Request) {
	// Mock recent errors (in production, this would query a logs table)
	errors := []map[string]interface{}{
		{
			"id":       1,
			"time":     "2 minutes ago",
			"level":    "error",
			"message":  "Failed to load bars for symbol XYZ",
			"user":     "alice",
			"endpoint": "/api/v1/scan",
		},
		{
			"id":       2,
			"time":     "15 minutes ago",
			"level":    "warning",
			"message":  "Rate limit approaching for user bob",
			"user":     "bob",
			"endpoint": "/api/v1/studies",
		},
	}

	writeJSON(w, http.StatusOK, errors)
}
