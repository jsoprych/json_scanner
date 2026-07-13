package api

import (
	"encoding/json"
	"net/http"
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

// TranslateStudy converts natural language to a study via LLM.
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
