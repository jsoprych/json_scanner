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
// Enforces per-user NLP enabled check and daily limits.
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

	// Resolve user and check NLP permissions
	userID := "anonymous"
	if cookie, err := r.Cookie("cetus_session"); err == nil && h.validateSession != nil {
		if id, _, ok := h.validateSession(cookie.Value); ok {
			userID = id
		}
	}

	today := time.Now().Format("2006-01-02")

	// Per-user NLP enabled check
	if u, ok := h.users.Find(userID); ok {
		if !u.NLPEnabled {
			writeError(w, http.StatusForbidden, "NLP_DISABLED_USER", "NLP is not enabled for your account")
			return
		}
		// Per-user daily limit
		if u.NLPDailyLimit > 0 && h.throttler != nil {
			var count int
			h.throttler.DB().QueryRow(
				"SELECT COALESCE(replays_run,0) FROM usage_tracking WHERE user_id=? AND date=?",
				userID, today,
			).Scan(&count)
			if count >= u.NLPDailyLimit {
				writeError(w, http.StatusTooManyRequests, "NLP_LIMIT",
					fmt.Sprintf("Daily NLP limit reached (%d/%d). Try tomorrow.", count, u.NLPDailyLimit))
				return
			}
		}
	}

	// System-wide cap
	if h.throttler != nil {
		var sysCount int
		h.throttler.DB().QueryRow(
			"SELECT COALESCE(replays_run,0) FROM usage_tracking WHERE user_id='system' AND date=?",
			today,
		).Scan(&sysCount)
		if sysCount >= nlpDailyCap {
			writeError(w, http.StatusTooManyRequests, "SYSTEM_CAP",
				fmt.Sprintf("System NLP cap reached (%d). Try tomorrow.", nlpDailyCap))
			return
		}
	}

	result, err := h.nlp.Translate(req.Query)
	if err != nil {
		writeJSON(w, http.StatusOK, TranslateStudyResponse{Error: err.Error()})
		return
	}

	// Track usage
	if h.throttler != nil {
		h.throttler.DB().Exec(
			"INSERT INTO usage_tracking (user_id, date, replays_run) VALUES (?, ?, 1) ON CONFLICT(user_id, date) DO UPDATE SET replays_run = replays_run + 1",
			userID, today,
		)
		h.throttler.DB().Exec(
			"INSERT INTO usage_tracking (user_id, date, replays_run) VALUES ('system', ?, 1) ON CONFLICT(user_id, date) DO UPDATE SET replays_run = replays_run + 1",
			today,
		)
	}

	writeJSON(w, http.StatusOK, TranslateStudyResponse{
		Where:   result.Where,
		OrderBy: result.OrderBy,
		Limit:   result.Limit,
	})
}
