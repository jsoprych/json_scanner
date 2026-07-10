package api

import (
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
