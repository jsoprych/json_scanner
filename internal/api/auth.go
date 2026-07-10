package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"cetus-marketdata-scanner/internal/authjwt"
	"cetus-marketdata-scanner/internal/user"
)

// LoginRequest is the request body for POST /auth/login.
type LoginRequest struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

// LoginResponse is the response for POST /auth/login.
type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	User      string `json:"user"`
	Tier      string `json:"tier"`
	Role      string `json:"role"`
}

// Login authenticates a user and returns a JWT.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if h.users == nil || h.signer == nil {
		writeError(w, http.StatusServiceUnavailable, "AUTH_DISABLED", "authentication not configured")
		return
	}
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	u, ok := h.users.Find(req.User)
	if !ok || !u.CheckPassword(req.Password) {
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid user or password")
		return
	}
	if u.Disabled {
		writeError(w, http.StatusForbidden, "ACCOUNT_DISABLED", "account is disabled")
		return
	}

	exp := time.Now().Add(h.signer.TTL()).Unix()
	claims := authjwt.Claims{
		Sub:  u.ID,
		Exp:  exp,
		Tier: string(u.Tier),
		Role: string(u.Role),
	}
	token, err := h.signer.Sign(claims)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SIGN_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, LoginResponse{
		Token:     token,
		ExpiresAt: exp,
		User:      u.ID,
		Tier:      string(u.Tier),
		Role:      string(u.Role),
	})
}

// Me returns the current user from the JWT.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	u, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// requireAuth extracts the user from the JWT in the Authorization header.
func (h *Handler) requireAuth(w http.ResponseWriter, r *http.Request) (user.User, bool) {
	if h.verifier == nil {
		writeError(w, http.StatusServiceUnavailable, "AUTH_DISABLED", "authentication not configured")
		return user.User{}, false
	}
	auth := r.Header.Get("Authorization")
	if auth == "" {
		writeError(w, http.StatusUnauthorized, "MISSING_TOKEN", "Authorization header required")
		return user.User{}, false
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	if token == auth {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Bearer token required")
		return user.User{}, false
	}

	userID, err := h.verifier.Verify(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", err.Error())
		return user.User{}, false
	}

	if h.users == nil {
		writeError(w, http.StatusServiceUnavailable, "AUTH_DISABLED", "user store not configured")
		return user.User{}, false
	}
	u, ok := h.users.Find(userID)
	if !ok {
		writeError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "user not found")
		return user.User{}, false
	}
	if u.Disabled {
		writeError(w, http.StatusForbidden, "ACCOUNT_DISABLED", "account is disabled")
		return user.User{}, false
	}

	return u, true
}
