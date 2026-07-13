package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TranslateStudyRequest is the request body for NLP translation.
type TranslateStudyRequest struct {
	Query string `json:"query"`
}

// TranslateStudyResponse is the response.
type TranslateStudyResponse struct {
	Where   string `json:"where"`
	OrderBy string `json:"order_by"`
	Limit   int    `json:"limit"`
	Error   string `json:"error,omitempty"`
}

// nlpDailyCap is the hard system-wide daily cap for NLP translations.
const nlpDailyCap = 1000

// TranslateStudy converts natural language to a study via LLM.
// Hard-capped at 1000/day system-wide.
func (h *Handler) TranslateStudy(w http.ResponseWriter, r *http.Request) {
	if h.nlp == nil {
		writeError(w, http.StatusServiceUnavailable, "NLP_DISABLED", "NLP translator not configured")
		return
	}

	var req TranslateStudyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "MISSING_QUERY", "query is required")
		return
	}

	// System-wide daily cap using throttler's tracking table
	if h.throttler != nil {
		today := time.Now().Format("2006-01-02")
		var count int
		h.throttler.DB().QueryRow(
			"SELECT COALESCE(replays_run,0) FROM usage_tracking WHERE user_id='system' AND date=?",
			today,
		).Scan(&count)
		if count >= nlpDailyCap {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED",
				fmt.Sprintf("Daily NLP cap reached (%d). Try tomorrow.", nlpDailyCap))
			return
		}
		h.throttler.DB().Exec(
			"INSERT INTO usage_tracking (user_id, date, replays_run) VALUES ('system', ?, 1) ON CONFLICT(user_id, date) DO UPDATE SET replays_run = replays_run + 1",
			today,
		)
	}

	result, err := h.nlp.Translate(req.Query)
	if err != nil {
		writeJSON(w, http.StatusOK, TranslateStudyResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, TranslateStudyResponse{
		Where:   result.Where,
		OrderBy: result.OrderBy,
		Limit:   result.Limit,
	})
}
