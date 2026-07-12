package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// ResetPassword generates a temporary password and forces change on next login.
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	u, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	if !u.IsAdmin() {
		writeError(w, http.StatusForbidden, "ADMIN_ONLY", "admin access required")
		return
	}
	if h.users == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_USERS", "user store not configured")
		return
	}

	userID := r.PathValue("id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_ID", "user id required")
		return
	}

	// Generate temporary password (12 random chars)
	b := make([]byte, 8)
	rand.Read(b)
	tempPW := hex.EncodeToString(b)[:12]

	// Set the temp password (SetPassword hashes it)
	if err := h.users.SetPassword(userID, tempPW); err != nil {
		writeError(w, http.StatusInternalServerError, "RESET_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":        "reset",
		"temp_password": tempPW,
	})
}
