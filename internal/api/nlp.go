package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cetus-marketdata-scanner/internal/nlp"
)

// TranslateStudyRequest is the request body for NLP translation.
type TranslateStudyRequest struct {
	Query string `json:"query"`
}

// TranslateStudyResponse is the response for NLP translation.
type TranslateStudyResponse struct {
	Where       string `json:"where"`
	OrderBy     string `json:"order_by"`
	Limit       int    `json:"limit"`
	Error       string `json:"error,omitempty"`
	ErrorDetail string `json:"error_detail,omitempty"` // human-readable explanation
	Remaining   int    `json:"remaining,omitempty"`    // remaining daily quota
	UpgradeHint string `json:"upgrade_hint,omitempty"` // how to get more access
}

const (
	nlpSystemDailyCap = 1000
	nlpFreeDailyCap   = 10
)

// nlperr writes a structured NLP error response (always HTTP 200 for UI handling).
func nlperr(w http.ResponseWriter, msg, detail, upgrade string) {
	writeJSON(w, http.StatusOK, TranslateStudyResponse{
		Error:       msg,
		ErrorDetail: detail,
		UpgradeHint: upgrade,
	})
}

// TranslateStudy converts natural language to a study via LLM.
func (h *Handler) TranslateStudy(w http.ResponseWriter, r *http.Request) {
	if h.nlp == nil {
		nlperr(w, "NLP_DISABLED",
			"The NLP translator is not configured on this server.",
			"Contact your administrator to enable the AI study translator.")
		return
	}

	var req TranslateStudyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		nlperr(w, "BAD_REQUEST", "Could not parse the request body.", "")
		return
	}
	if req.Query == "" {
		nlperr(w, "EMPTY_QUERY",
			"Please describe what you're looking for in plain English.",
			`Example: "stocks above 200 DMA with RSI between 55 and 70, most liquid first"`)
		return
	}

	// Resolve user
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
			nlperr(w, "NLP_DISABLED_USER",
				fmt.Sprintf("NLP translations are not enabled for '%s'.", u.ID),
				"Ask your admin to enable NLP for your account in the user settings.")
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
				nlperr(w, "USER_QUOTA_EXCEEDED",
					fmt.Sprintf("You've used %d of %d daily NLP translations.", count, u.NLPDailyLimit),
					"Quota resets at midnight UTC. Contact your admin to increase your daily limit.")
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
		if sysCount >= nlpSystemDailyCap {
			nlperr(w, "SYSTEM_CAP_REACHED",
				fmt.Sprintf("System-wide NLP cap of %d reached for today.", nlpSystemDailyCap),
				"Try again tomorrow after midnight UTC.")
			return
		}
	}

	result, err := h.nlp.Translate(req.Query)
	if err != nil {
		nlperr(w, "TRANSLATION_FAILED",
			"The AI could not understand that query. Try rephrasing.",
			`Try shorter, more specific queries like "stocks above 50 DMA with RSI oversold"`)
		return
	}

	// Final gate: validate SQL against real SQLite schema (LIMIT 0 — free)
	if err := nlp.ValidateSQL(h.snap.LogDB(), result.Where, result.OrderBy); err != nil {
		nlperr(w, "SQL_INVALID",
			fmt.Sprintf("The generated SQL is invalid: %s", err.Error()),
			"The AI produced SQL that doesn't match the database schema. Try rephrasing your query.")
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
