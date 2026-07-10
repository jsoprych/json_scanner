package api

import (
	"net/http"
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

	// Auth endpoints
	mux.HandleFunc("POST /api/v1/auth/login", h.Login)
	mux.HandleFunc("GET /api/v1/auth/me", h.Me)

	// User management (admin)
	mux.HandleFunc("GET /api/v1/users", h.ListUsers)
	mux.HandleFunc("POST /api/v1/users", h.CreateUser)
	mux.HandleFunc("GET /api/v1/users/{id}", h.GetUser)
	mux.HandleFunc("PUT /api/v1/users/{id}", h.UpdateUser)
	mux.HandleFunc("DELETE /api/v1/users/{id}", h.DeleteUser)

	return mux
}
