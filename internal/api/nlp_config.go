package api

import (
	"encoding/json"
	"net/http"
)

// GetUserNLPConfig returns NLP config for a user.
func (h *Handler) GetUserNLPConfig(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_ID", "user id required")
		return
	}
	u, ok := h.users.Find(userID)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"nlp_enabled":     u.NLPEnabled,
		"nlp_daily_limit": u.NLPDailyLimit,
	})
}

// SetUserNLPConfig updates NLP config for a user (admin only).
func (h *Handler) SetUserNLPConfig(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	userID := r.PathValue("id")
	var req struct {
		NLPEnabled    *bool `json:"nlp_enabled"`
		NLPDailyLimit *int  `json:"nlp_daily_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	if req.NLPEnabled != nil {
		if err := h.users.SetNLPEnabled(userID, *req.NLPEnabled); err != nil {
			writeError(w, http.StatusInternalServerError, "FAILED", err.Error())
			return
		}
	}
	if req.NLPDailyLimit != nil {
		if err := h.users.SetNLPDailyLimit(userID, *req.NLPDailyLimit); err != nil {
			writeError(w, http.StatusInternalServerError, "FAILED", err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetNLPDailyCount returns today's NLP usage count for a user.
func (h *Handler) GetNLPDailyCount(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	userID := r.PathValue("id")
	var count int
	// Query the usage tracking table (uses replays_run column for NLP count)
	_ = h.throttler.DB().QueryRow(
		"SELECT COALESCE(replays_run,0) FROM usage_tracking WHERE user_id=? AND date=date('now')",
		userID,
	).Scan(&count)
	writeJSON(w, http.StatusOK, map[string]int{"count": count})
}

// SetNLPEnabled is a helper for the user store.

// unused imports cleanup
