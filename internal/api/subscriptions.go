package api

import (
	"net/http"

	"cetus-marketdata-scanner/internal/study"
)

// Subscriptions returns all studies the authenticated user is subscribed to.
func (h *Handler) Subscriptions(w http.ResponseWriter, r *http.Request) {
	if h.subs == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_SUBSCRIPTIONS", "subscription store not configured")
		return
	}

	u, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	studyKeys := h.subs.GetUserSubscriptions(u.ID)
	
	// Get the actual study objects
	var studies []study.Study
	for _, key := range studyKeys {
		if s, exists := h.studies.Get(key); exists {
			studies = append(studies, s)
		}
	}

	writeJSON(w, http.StatusOK, studies)
}

// SubscribeRequest is the request body for subscribing to a study.
type SubscribeRequest struct {
	StudyKey string `json:"study_key"`
}

// Subscribe adds a subscription for the authenticated user to a study.
func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	if h.subs == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_SUBSCRIPTIONS", "subscription store not configured")
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

	// Check if study exists
	if _, exists := h.studies.Get(studyKey); !exists {
		writeError(w, http.StatusNotFound, "STUDY_NOT_FOUND", "study not found")
		return
	}

	// Subscribe the user
	if err := h.subs.Subscribe(u.ID, studyKey); err != nil {
		writeError(w, http.StatusInternalServerError, "SUBSCRIBE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":    "subscribed",
		"user_id":   u.ID,
		"study_key": studyKey,
	})
}

// Unsubscribe removes a subscription for the authenticated user from a study.
func (h *Handler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	if h.subs == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_SUBSCRIPTIONS", "subscription store not configured")
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

	// Unsubscribe the user
	if err := h.subs.Unsubscribe(u.ID, studyKey); err != nil {
		writeError(w, http.StatusInternalServerError, "UNSUBSCRIBE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":    "unsubscribed",
		"user_id":   u.ID,
		"study_key": studyKey,
	})
}

// IsSubscribed checks if the authenticated user is subscribed to a study.
func (h *Handler) IsSubscribed(w http.ResponseWriter, r *http.Request) {
	if h.subs == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_SUBSCRIPTIONS", "subscription store not configured")
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

	isSubscribed := h.subs.IsSubscribed(u.ID, studyKey)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":       u.ID,
		"study_key":     studyKey,
		"is_subscribed": isSubscribed,
	})
}

// StudySubscribers returns all users subscribed to a study (admin only).
func (h *Handler) StudySubscribers(w http.ResponseWriter, r *http.Request) {
	if h.subs == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_SUBSCRIPTIONS", "subscription store not configured")
		return
	}

	u, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	if !u.IsAdmin() {
		writeError(w, http.StatusForbidden, "ADMIN_ONLY", "only admins can view study subscribers")
		return
	}

	studyKey := r.PathValue("id")
	if studyKey == "" {
		writeError(w, http.StatusBadRequest, "MISSING_STUDY_KEY", "study key is required")
		return
	}

	userIDs := h.subs.GetStudySubscribers(studyKey)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"study_key":    studyKey,
		"subscribers":  userIDs,
		"total_count":  len(userIDs),
	})
}
