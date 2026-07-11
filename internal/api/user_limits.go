package api

import (
	"encoding/json"
	"net/http"

	"cetus-marketdata-scanner/internal/roles"
)

// GetUserLimits returns effective limits for a user
func (h *Handler) GetUserLimits(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	userID := r.PathValue("user_id")

	// Users can only view their own limits unless admin
	if userID != user.ID {
		hasCap, err := h.roles.HasCapability(user.RoleID, "system.admin")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "CHECK_FAILED", err.Error())
			return
		}
		if !hasCap {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
			return
		}
	}

	limits, err := h.throttler.GetEffectiveLimits(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, limits)
}

// SetUserLimits sets user-specific limit overrides (admin only)
func (h *Handler) SetUserLimits(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	// Check admin capability
	hasCap, err := h.roles.HasCapability(user.RoleID, "system.admin")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CHECK_FAILED", err.Error())
		return
	}
	if !hasCap {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	userID := r.PathValue("user_id")

	var limits roles.Limits
	if err := json.NewDecoder(r.Body).Decode(&limits); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	if err := h.throttler.SetUserLimits(userID, limits); err != nil {
		writeError(w, http.StatusInternalServerError, "SET_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "updated",
	})
}

// GetUserUsage returns current usage for a user
func (h *Handler) GetUserUsage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	userID := r.PathValue("user_id")

	// Users can only view their own usage unless admin
	if userID != user.ID {
		hasCap, err := h.roles.HasCapability(user.RoleID, "system.admin")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "CHECK_FAILED", err.Error())
			return
		}
		if !hasCap {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
			return
		}
	}

	usage, err := h.throttler.GetUserUsage(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, usage)
}
