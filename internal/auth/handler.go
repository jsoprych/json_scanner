package auth

import (
	"encoding/json"
	"net/http"

	"cetus-marketdata-scanner/internal/user"
)

// ChangePasswordRequest is the request body for password change.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePasswordResponse is the response.
type ChangePasswordResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// HandleChangePassword processes a password change request.
// users: the user store. policy: password strength rules.
// Returns a reusable http.HandlerFunc.
func HandleChangePassword(users *user.Store, policy PasswordPolicy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut && r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ChangePasswordResponse{Error: "PUT or POST required"})
			return
		}

		// Get current user from context (set by auth middleware)
		currentUser, ok := r.Context().Value("user").(user.User)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, ChangePasswordResponse{Error: "not authenticated"})
			return
		}

		var req ChangePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ChangePasswordResponse{Error: "invalid request body"})
			return
		}

		// Verify current password
		if !currentUser.CheckPassword(req.CurrentPassword) {
			writeJSON(w, http.StatusUnauthorized, ChangePasswordResponse{Error: "current password is incorrect"})
			return
		}

		// Validate new password
		if req.CurrentPassword == req.NewPassword {
			writeJSON(w, http.StatusBadRequest, ChangePasswordResponse{Error: "new password must differ from current"})
			return
		}
		if err := ValidatePassword(req.NewPassword, policy); err != nil {
			writeJSON(w, http.StatusBadRequest, ChangePasswordResponse{Error: err.Error()})
			return
		}

		// Update password
		if err := users.SetPassword(currentUser.ID, req.NewPassword); err != nil {
			writeJSON(w, http.StatusInternalServerError, ChangePasswordResponse{Error: "failed to update password"})
			return
		}

		writeJSON(w, http.StatusOK, ChangePasswordResponse{Status: "password changed"})
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
