package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"cetus-marketdata-scanner/internal/features"
	"cetus-marketdata-scanner/internal/snapshot"
)

// Handler provides REST API endpoints.
type Handler struct {
	snap   *snapshot.DB
	log    *slog.Logger
	start  time.Time
}

// NewHandler creates a new API handler.
func NewHandler(snap *snapshot.DB, log *slog.Logger) *Handler {
	return &Handler{
		snap:  snap,
		log:   log,
		start: time.Now(),
	}
}

// HealthResponse is the response for /health endpoint.
type HealthResponse struct {
	Status      string `json:"status"`
	SnapshotID  string `json:"snapshot_id"`
	SymbolCount int    `json:"symbol_count"`
	Uptime      string `json:"uptime"`
}

// Health returns server health status.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	meta := h.snap.Metadata()
	
	resp := HealthResponse{
		Status:      "ok",
		SnapshotID:  meta.SnapshotID,
		SymbolCount: meta.SymbolCount,
		Uptime:      time.Since(h.start).Round(time.Second).String(),
	}
	
	writeJSON(w, http.StatusOK, resp)
}

// FeaturesResponse is the response for /features endpoint.
type FeaturesResponse struct {
	Features []features.Feature `json:"features"`
	Count    int                `json:"count"`
}

// Features returns the catalog of available indicators.
func (h *Handler) Features(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	
	var feats []features.Feature
	if category != "" {
		feats = features.ByCategory(category)
	} else {
		feats = features.Registry()
	}
	
	resp := FeaturesResponse{
		Features: feats,
		Count:    len(feats),
	}
	
	writeJSON(w, http.StatusOK, resp)
}

// FeatureByID returns a single feature by ID.
func (h *Handler) FeatureByID(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path: /features/{id}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Feature ID required")
		return
	}
	
	feat := features.ByID(id)
	if feat == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Feature not found")
		return
	}
	
	writeJSON(w, http.StatusOK, feat)
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// ErrorResponse is the error response format.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{
		Error: message,
		Code:  code,
	})
}
