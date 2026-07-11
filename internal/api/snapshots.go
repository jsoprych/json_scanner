package api

import (
	"encoding/json"
	"net/http"
)

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

// SetActiveSnapshotRequest is the request for setting active snapshot.
type SetActiveSnapshotRequest struct {
	Date int64 `json:"date"`
}

// SetActiveSnapshot sets the active snapshot date for queries.
func (h *Handler) SetActiveSnapshot(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAuth(w, r); !ok {
		return
	}

	var req SetActiveSnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	if req.Date == 0 {
		writeError(w, http.StatusBadRequest, "MISSING_DATE", "date is required")
		return
	}

	// Check if snapshot exists
	exists, err := h.snap.HasSnapshot(req.Date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CHECK_FAILED", err.Error())
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "snapshot not found for date")
		return
	}

	// Set active snapshot
	if err := h.snap.SetActive(req.Date); err != nil {
		writeError(w, http.StatusInternalServerError, "SET_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "ok",
		"active_date": req.Date,
	})
}
