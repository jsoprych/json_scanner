package api

import (
	"net/http"
	"strconv"
)

// Backtest runs a historical backtest for a study.
func (h *Handler) Backtest(w http.ResponseWriter, r *http.Request) {
	if h.backtest == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_BACKTEST", "backtest engine not configured")
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

	// Parse query parameters
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")
	holdDaysStr := r.URL.Query().Get("hold_days")

	if startDateStr == "" || endDateStr == "" {
		writeError(w, http.StatusBadRequest, "MISSING_DATES", "start_date and end_date are required")
		return
	}

	startDate, err := strconv.ParseInt(startDateStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_START_DATE", "start_date must be a Unix timestamp")
		return
	}

	endDate, err := strconv.ParseInt(endDateStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_END_DATE", "end_date must be a Unix timestamp")
		return
	}

	holdDays := 20 // Default hold period
	if holdDaysStr != "" {
		holdDays, err = strconv.Atoi(holdDaysStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_HOLD_DAYS", "hold_days must be an integer")
			return
		}
	}

	// Run the backtest
	summary, err := h.backtest.RunBacktest(s, startDate, endDate, holdDays)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "BACKTEST_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// PointInTime runs a study on a specific historical date.
func (h *Handler) PointInTime(w http.ResponseWriter, r *http.Request) {
	if h.backtest == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_BACKTEST", "backtest engine not configured")
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
	if dateStr == "" {
		writeError(w, http.StatusBadRequest, "MISSING_DATE", "date is required")
		return
	}

	date, err := strconv.ParseInt(dateStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_DATE", "date must be a Unix timestamp")
		return
	}

	matches, err := h.backtest.RunPointInTime(s, date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "POINT_IN_TIME_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"study_key": studyKey,
		"date":      date,
		"matches":   matches,
		"count":     len(matches),
	})
}
