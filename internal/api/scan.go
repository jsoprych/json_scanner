package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"cetus-marketdata-scanner/internal/snapshot"
	"cetus-marketdata-scanner/internal/study"
)

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
