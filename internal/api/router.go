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

	// Scan (to be implemented)
	// mux.HandleFunc("GET /api/v1/scan", h.Scan)
	// mux.HandleFunc("POST /api/v1/scan", h.Scan)

	// Studies (to be implemented)
	// mux.HandleFunc("GET /api/v1/studies", h.ListStudies)
	// mux.HandleFunc("POST /api/v1/studies", h.CreateStudy)
	// mux.HandleFunc("GET /api/v1/studies/{id}", h.GetStudy)
	// mux.HandleFunc("PUT /api/v1/studies/{id}", h.UpdateStudy)
	// mux.HandleFunc("DELETE /api/v1/studies/{id}", h.DeleteStudy)

	// Symbols (to be implemented)
	// mux.HandleFunc("GET /api/v1/universe", h.Universe)
	// mux.HandleFunc("GET /api/v1/symbols/{symbol}", h.Symbol)

	return mux
}
