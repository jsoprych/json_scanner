package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"cetus-marketdata-scanner/internal/features"
	"cetus-marketdata-scanner/internal/model"
	"cetus-marketdata-scanner/internal/snapshot"
	"cetus-marketdata-scanner/internal/store"
	"cetus-marketdata-scanner/internal/study"
)

// UniverseLoader provides universe and symbol queries.
type UniverseLoader interface {
	Universe(ctx context.Context) ([]string, error)
	UniverseCommon(ctx context.Context) ([]string, error)
	LoadAdjustedBars(ctx context.Context, symbol string, since int64) ([]model.Bar, error)
}

// Handler provides REST API endpoints.
type Handler struct {
	snap    *snapshot.DB
	studies *study.Store
	store   *store.Store
	log     *slog.Logger
	start   time.Time
}

// NewHandler creates a new API handler.
func NewHandler(snap *snapshot.DB, log *slog.Logger) *Handler {
	return &Handler{
		snap:  snap,
		log:   log,
		start: time.Now(),
	}
}

// NewHandlerWithDeps creates a handler with full dependencies (studies, store).
func NewHandlerWithDeps(snap *snapshot.DB, studies *study.Store, st *store.Store, log *slog.Logger) *Handler {
	return &Handler{
		snap:    snap,
		studies: studies,
		store:   st,
		log:     log,
		start:   time.Now(),
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

// --- Scan endpoints ---

// ScanRequest is the request body for POST /scan.
type ScanRequest struct {
	Where   string `json:"where"`
	OrderBy string `json:"order_by,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// ScanResponse is the response for /scan endpoints.
type ScanResponse struct {
	Matches []snapshot.Match `json:"matches"`
	Count   int              `json:"count"`
}

// Scan runs an ad-hoc study against the current snapshot.
func (h *Handler) Scan(w http.ResponseWriter, r *http.Request) {
	var req ScanRequest
	switch r.Method {
	case http.MethodGet:
		req.Where = r.URL.Query().Get("where")
		req.OrderBy = r.URL.Query().Get("order_by")
		if limit := r.URL.Query().Get("limit"); limit != "" {
			if n, err := strconv.Atoi(limit); err == nil {
				req.Limit = n
			}
		}
	case http.MethodPost:
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
			return
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if req.Where == "" {
		writeError(w, http.StatusBadRequest, "MISSING_WHERE", "where clause required")
		return
	}

	matches, err := h.snap.Run(study.Study{Where: req.Where, OrderBy: req.OrderBy, Limit: req.Limit})
	if err != nil {
		writeError(w, http.StatusBadRequest, "SCAN_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ScanResponse{Matches: matches, Count: len(matches)})
}

// --- Studies endpoints ---

// ListStudies returns all studies (admin) or the user's accessible studies.
func (h *Handler) ListStudies(w http.ResponseWriter, r *http.Request) {
	if h.studies == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_STUDIES", "studies not configured")
		return
	}
	all := h.studies.All()
	writeJSON(w, http.StatusOK, all)
}

// CreateStudy creates a new study.
func (h *Handler) CreateStudy(w http.ResponseWriter, r *http.Request) {
	if h.studies == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_STUDIES", "studies not configured")
		return
	}
	var st study.Study
	if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if err := h.studies.Upsert(st); err != nil {
		writeError(w, http.StatusBadRequest, "CREATE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, st)
}

// GetStudy returns a study by key.
func (h *Handler) GetStudy(w http.ResponseWriter, r *http.Request) {
	if h.studies == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_STUDIES", "studies not configured")
		return
	}
	key := r.PathValue("id")
	st, ok := h.studies.Get(key)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "study not found")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// UpdateStudy updates an existing study.
func (h *Handler) UpdateStudy(w http.ResponseWriter, r *http.Request) {
	if h.studies == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_STUDIES", "studies not configured")
		return
	}
	key := r.PathValue("id")
	if _, ok := h.studies.Get(key); !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "study not found")
		return
	}
	var st study.Study
	if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	st.Key = key
	if err := h.studies.Upsert(st); err != nil {
		writeError(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// DeleteStudy deletes a study by key.
func (h *Handler) DeleteStudy(w http.ResponseWriter, r *http.Request) {
	if h.studies == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_STUDIES", "studies not configured")
		return
	}
	key := r.PathValue("id")
	if err := h.studies.Delete(key); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Universe/Symbol endpoints ---

// UniverseResponse is the response for /universe.
type UniverseResponse struct {
	Symbols []string `json:"symbols"`
	Count   int      `json:"count"`
}

// Universe returns the list of scannable symbols.
func (h *Handler) Universe(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_STORE", "warehouse not connected")
		return
	}
	symbols, err := h.store.Universe(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "UNIVERSE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, UniverseResponse{Symbols: symbols, Count: len(symbols)})
}

// SymbolResponse is the response for /symbols/{symbol}.
type SymbolResponse struct {
	Symbol string       `json:"symbol"`
	Bars   []model.Bar  `json:"bars"`
	Count  int          `json:"count"`
}

// Symbol returns recent bars for a symbol.
func (h *Handler) Symbol(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_STORE", "warehouse not connected")
		return
	}
	symbol := r.PathValue("symbol")
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "MISSING_SYMBOL", "symbol required")
		return
	}

	// Default to 120 days of history
	since := time.Now().UTC().AddDate(0, 0, -120).Unix()
	if s := r.URL.Query().Get("since"); s != "" {
		if days, err := strconv.Atoi(s); err == nil {
			since = time.Now().UTC().AddDate(0, 0, -days).Unix()
		}
	}

	bars, err := h.store.LoadAdjustedBars(r.Context(), symbol, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LOAD_FAILED", err.Error())
		return
	}
	if len(bars) == 0 {
		writeError(w, http.StatusNotFound, "NO_DATA", "no bars found for symbol")
		return
	}

	writeJSON(w, http.StatusOK, SymbolResponse{Symbol: symbol, Bars: bars, Count: len(bars)})
}

// --- Snapshot history endpoints ---

// SnapshotsResponse is the response for /snapshots.
type SnapshotsResponse struct {
	Dates []int64 `json:"dates"`
	Count int     `json:"count"`
}

// Snapshots returns the list of available snapshot dates.
func (h *Handler) Snapshots(w http.ResponseWriter, r *http.Request) {
	dates, err := h.snap.ListSnapshots()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, SnapshotsResponse{Dates: dates, Count: len(dates)})
}
