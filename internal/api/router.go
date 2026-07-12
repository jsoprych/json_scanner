package api

import (
	"net/http"
	"strings"
)

// Router returns an HTTP handler with all API routes mounted.
func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()

	// Health & metadata
	mux.HandleFunc("GET /api/v1/health", h.Health)

	// Features catalog
	mux.HandleFunc("GET /api/v1/features", h.Features)
	mux.HandleFunc("GET /api/v1/features/{id}", h.FeatureByID)

	// Scan (ad-hoc study)
	mux.HandleFunc("GET /api/v1/scan", h.Scan)
	mux.HandleFunc("POST /api/v1/scan", h.Scan)

	// Studies CRUD
	mux.HandleFunc("GET /api/v1/studies", h.ListStudies)
	mux.HandleFunc("POST /api/v1/studies", h.CreateStudy)
	mux.HandleFunc("GET /api/v1/studies/{id}", h.GetStudy)
	mux.HandleFunc("PUT /api/v1/studies/{id}", h.UpdateStudy)
	mux.HandleFunc("DELETE /api/v1/studies/{id}", h.DeleteStudy)

	// Study import/export
	mux.HandleFunc("POST /api/v1/studies/import", h.ImportStudies)
	mux.HandleFunc("GET /api/v1/studies/export", h.ExportStudies)

	// Study subscriptions
	mux.HandleFunc("GET /api/v1/subscriptions", h.Subscriptions)
	mux.HandleFunc("POST /api/v1/studies/{id}/subscribe", h.Subscribe)
	mux.HandleFunc("DELETE /api/v1/studies/{id}/subscribe", h.Unsubscribe)
	mux.HandleFunc("GET /api/v1/studies/{id}/subscribed", h.IsSubscribed)
	mux.HandleFunc("GET /api/v1/studies/{id}/subscribers", h.StudySubscribers)

	// Alert detection (entries/exits)
	mux.HandleFunc("GET /api/v1/studies/{id}/alerts", h.Alerts)
	mux.HandleFunc("GET /api/v1/studies/{id}/entries", h.Entries)
	mux.HandleFunc("GET /api/v1/studies/{id}/exits", h.Exits)

	// Backtest (historical queries)
	mux.HandleFunc("GET /api/v1/studies/{id}/backtest", h.Backtest)
	mux.HandleFunc("GET /api/v1/studies/{id}/pointintime", h.PointInTime)

	// Universe & symbols
	mux.HandleFunc("GET /api/v1/universe", h.Universe)
	mux.HandleFunc("GET /api/v1/symbols/{symbol}", h.Symbol)

	// Snapshot history
	mux.HandleFunc("GET /api/v1/snapshots", h.Snapshots)
	mux.HandleFunc("POST /api/v1/snapshots/active", h.SetActiveSnapshot)

	// Groups management
	mux.HandleFunc("POST /api/v1/groups", h.CreateGroup)
	mux.HandleFunc("GET /api/v1/groups", h.ListGroups)
	mux.HandleFunc("GET /api/v1/groups/{id}", h.GetGroup)
	mux.HandleFunc("DELETE /api/v1/groups/{id}", h.DeleteGroup)
	mux.HandleFunc("GET /api/v1/groups/{id}/members", h.ListGroupMembers)
	mux.HandleFunc("POST /api/v1/groups/{id}/members", h.AddGroupMember)
	mux.HandleFunc("DELETE /api/v1/groups/{id}/members/{user_id}", h.RemoveGroupMember)
	mux.HandleFunc("GET /api/v1/users/{user_id}/groups", h.ListUserGroups)

	// Saved results with Linux-style permissions
	mux.HandleFunc("POST /api/v1/results", h.SaveResult)
	mux.HandleFunc("GET /api/v1/results", h.ListResults)
	mux.HandleFunc("GET /api/v1/results/accessible", h.ListAccessibleResults)
	mux.HandleFunc("GET /api/v1/results/export", h.ExportResults)
	mux.HandleFunc("POST /api/v1/results/import", h.ImportResults)
	mux.HandleFunc("GET /api/v1/results/{id}", h.GetResult)
	mux.HandleFunc("DELETE /api/v1/results/{id}", h.DeleteResult)
	mux.HandleFunc("PATCH /api/v1/results/{id}/permissions", h.UpdateResultPermissions)
	mux.HandleFunc("PATCH /api/v1/results/{id}/owner", h.ChangeResultOwner)
	mux.HandleFunc("GET /api/v1/results/{id}/acls", h.ListResultACLs)
	mux.HandleFunc("POST /api/v1/results/{id}/acls", h.AddResultACL)
	mux.HandleFunc("DELETE /api/v1/results/{id}/acls/{user_id}", h.RemoveResultACL)

	// Auth endpoints
	mux.HandleFunc("POST /api/v1/auth/login", h.Login)
	mux.HandleFunc("GET /api/v1/auth/me", h.Me)

	// User management (admin)
	mux.HandleFunc("GET /api/v1/users", h.ListUsers)
	mux.HandleFunc("POST /api/v1/users", h.CreateUser)
	mux.HandleFunc("GET /api/v1/users/{id}", h.GetUser)
	mux.HandleFunc("PUT /api/v1/users/{id}", h.UpdateUser)
	mux.HandleFunc("DELETE /api/v1/users/{id}", h.DeleteUser)

	// Roles management (admin)
	mux.HandleFunc("GET /api/v1/roles", h.ListRoles)
	mux.HandleFunc("POST /api/v1/roles", h.CreateRole)
	mux.HandleFunc("GET /api/v1/roles/{id}", h.GetRole)
	mux.HandleFunc("PUT /api/v1/roles/{id}", h.UpdateRole)
	mux.HandleFunc("DELETE /api/v1/roles/{id}", h.DeleteRole)

	// User limits and usage (admin or self)
	mux.HandleFunc("GET /api/v1/users/{user_id}/limits", h.GetUserLimits)
	mux.HandleFunc("PUT /api/v1/users/{user_id}/limits", h.SetUserLimits)
	mux.HandleFunc("GET /api/v1/users/{user_id}/usage", h.GetUserUsage)

	// Users — import/export
	mux.HandleFunc("GET /api/v1/users/export", h.ExportUsers)
	mux.HandleFunc("POST /api/v1/users/import", h.ImportUsers)

	// Groups — import/export
	mux.HandleFunc("GET /api/v1/groups/export", h.ExportGroups)
	mux.HandleFunc("POST /api/v1/groups/import", h.ImportGroups)

	// Roles — import/export
	mux.HandleFunc("GET /api/v1/roles/export", h.ExportRoles)
	mux.HandleFunc("POST /api/v1/roles/import", h.ImportRoles)

	// Wrap with usage tracking middleware
	return h.trackUsage(mux)
}

// trackUsage wraps an http.Handler with per-request usage tracking.
func (h *Handler) trackUsage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip health endpoint
		if r.URL.Path == "/api/v1/health" {
			next.ServeHTTP(w, r)
			return
		}
		// Try to identify user from session cookie or JWT
		userID := ""
		if cookie, err := r.Cookie("cetus_session"); err == nil && h.validateSession != nil {
			if id, _, ok := h.validateSession(cookie.Value); ok {
				userID = id
			}
		}
		if userID == "" {
			// Try JWT Bearer
			if auth := r.Header.Get("Authorization"); auth != "" {
				if h.verifier != nil {
					token := strings.TrimPrefix(auth, "Bearer ")
					if id, err := h.verifier.Verify(token); err == nil {
						userID = id
					}
				}
			}
		}
		next.ServeHTTP(w, r)
		// Track after serving so response time is accurate
		if userID != "" && h.throttler != nil {
			h.throttler.TrackAPIUsage(userID)
		}
	})
}
