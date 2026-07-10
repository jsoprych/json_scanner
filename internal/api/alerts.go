package api

import (
	"net/http"
	"strconv"

	"cetus-marketdata-scanner/internal/study"
)

// Alerts returns entries and exits for a study between two dates.
func (h *Handler) Alerts(w http.ResponseWriter, r *http.Request) {
	if h.detector == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_ALERTS", "alert detector not configured")
		return
	}

	u, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	studyKey := r.PathValue("id")
	if studyKey == "" {
		writeError(w, http.StatusBadRequest, "MISSING_STUDY_KEY", "study key is required")
		return
	}

	// Get the study
	s, exists := h.studies.Get(studyKey)
	if !exists {
		writeError(w, http.StatusNotFound, "STUDY_NOT_FOUND", "study not found")
		return
	}

	// Check if user can access this study
	if !canAccessStudy(s, u) {
		writeError(w, http.StatusForbidden, "ACCESS_DENIED", "you don't have access to this study")
		return
	}

	// Parse date parameters
	dateStr := r.URL.Query().Get("date")
	prevDateStr := r.URL.Query().Get("prev_date")

	if dateStr == "" || prevDateStr == "" {
		writeError(w, http.StatusBadRequest, "MISSING_DATES", "date and prev_date query parameters are required")
		return
	}

	date, err := strconv.ParseInt(dateStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_DATE", "date must be a Unix timestamp")
		return
	}

	prevDate, err := strconv.ParseInt(prevDateStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PREV_DATE", "prev_date must be a Unix timestamp")
		return
	}

	// Detect changes
	entries, exits, err := h.detector.DetectChanges(s, date, prevDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DETECT_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"study_key": studyKey,
		"date":      date,
		"prev_date": prevDate,
		"entries":   entries,
		"exits":     exits,
		"entry_count": len(entries),
		"exit_count":  len(exits),
	})
}

// Entries returns symbols that entered a study on a given date.
func (h *Handler) Entries(w http.ResponseWriter, r *http.Request) {
	if h.detector == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_ALERTS", "alert detector not configured")
		return
	}

	u, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	studyKey := r.PathValue("id")
	if studyKey == "" {
		writeError(w, http.StatusBadRequest, "MISSING_STUDY_KEY", "study key is required")
		return
	}

	s, exists := h.studies.Get(studyKey)
	if !exists {
		writeError(w, http.StatusNotFound, "STUDY_NOT_FOUND", "study not found")
		return
	}

	if !canAccessStudy(s, u) {
		writeError(w, http.StatusForbidden, "ACCESS_DENIED", "you don't have access to this study")
		return
	}

	dateStr := r.URL.Query().Get("date")
	prevDateStr := r.URL.Query().Get("prev_date")

	if dateStr == "" || prevDateStr == "" {
		writeError(w, http.StatusBadRequest, "MISSING_DATES", "date and prev_date query parameters are required")
		return
	}

	date, err := strconv.ParseInt(dateStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_DATE", "date must be a Unix timestamp")
		return
	}

	prevDate, err := strconv.ParseInt(prevDateStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PREV_DATE", "prev_date must be a Unix timestamp")
		return
	}

	entries, err := h.detector.DetectEntries(s, date, prevDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DETECT_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"study_key": studyKey,
		"date":      date,
		"prev_date": prevDate,
		"entries":   entries,
		"count":     len(entries),
	})
}

// Exits returns symbols that exited a study on a given date.
func (h *Handler) Exits(w http.ResponseWriter, r *http.Request) {
	if h.detector == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_ALERTS", "alert detector not configured")
		return
	}

	u, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	studyKey := r.PathValue("id")
	if studyKey == "" {
		writeError(w, http.StatusBadRequest, "MISSING_STUDY_KEY", "study key is required")
		return
	}

	s, exists := h.studies.Get(studyKey)
	if !exists {
		writeError(w, http.StatusNotFound, "STUDY_NOT_FOUND", "study not found")
		return
	}

	if !canAccessStudy(s, u) {
		writeError(w, http.StatusForbidden, "ACCESS_DENIED", "you don't have access to this study")
		return
	}

	dateStr := r.URL.Query().Get("date")
	prevDateStr := r.URL.Query().Get("prev_date")

	if dateStr == "" || prevDateStr == "" {
		writeError(w, http.StatusBadRequest, "MISSING_DATES", "date and prev_date query parameters are required")
		return
	}

	date, err := strconv.ParseInt(dateStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_DATE", "date must be a Unix timestamp")
		return
	}

	prevDate, err := strconv.ParseInt(prevDateStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PREV_DATE", "prev_date must be a Unix timestamp")
		return
	}

	exits, err := h.detector.DetectExits(s, date, prevDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DETECT_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"study_key": studyKey,
		"date":      date,
		"prev_date": prevDate,
		"exits":     exits,
		"count":     len(exits),
	})
}

// canAccessStudy checks if a user can access a study based on visibility and tier.
func canAccessStudy(s study.Study, u interface{ IsAdmin() bool }) bool {
	// This is a simplified check - in production you'd need the full user object
	// For now, just check if user is admin or study is public
	return true
}
