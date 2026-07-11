package api

import (
	"encoding/json"
	"net/http"

	"cetus-marketdata-scanner/internal/user"
)

// ListUsers returns all users (admin only).
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if h.users == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_USERS", "user store not configured")
		return
	}
	u, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	if !u.IsAdmin() {
		writeError(w, http.StatusForbidden, "ADMIN_ONLY", "admin access required")
		return
	}
	users := h.users.All()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"users": users,
		"count": len(users),
	})
}

// GetUser returns a user by ID.
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	if h.users == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_USERS", "user store not configured")
		return
	}
	authUser, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	// Users can view themselves, admins can view anyone
	if !authUser.IsAdmin() && authUser.ID != id {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "can only view your own profile")
		return
	}
	u, exists := h.users.Find(id)
	if !exists {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// CreateUserRequest is the request body for POST /users.
type CreateUserRequest struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Password string     `json:"password"`
	Tier     user.Tier  `json:"tier,omitempty"`
	Role     user.Role  `json:"role,omitempty"`
	Groups   []string   `json:"groups,omitempty"`
}

// CreateUser creates a new user (admin only).
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if h.users == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_USERS", "user store not configured")
		return
	}
	u, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	if !u.IsAdmin() {
		writeError(w, http.StatusForbidden, "ADMIN_ONLY", "admin access required")
		return
	}
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if req.ID == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "id and password required")
		return
	}
	newUser := user.User{
		ID:     req.ID,
		Name:   req.Name,
		Tier:   req.Tier,
		Role:   req.Role,
		Groups: req.Groups,
	}
	newUser.SetPassword(req.Password)
	if err := h.users.Create(newUser); err != nil {
		writeError(w, http.StatusBadRequest, "CREATE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, newUser)
}

// UpdateUserRequest is the request body for PUT /users/{id}.
type UpdateUserRequest struct {
	Name     string    `json:"name,omitempty"`
	Password string    `json:"password,omitempty"`
	Tier     user.Tier `json:"tier,omitempty"`
	Role     user.Role `json:"role,omitempty"`
	Groups   []string  `json:"groups,omitempty"`
	Disabled *bool     `json:"disabled,omitempty"`
}

// UpdateUser updates a user (admin only, or self for limited fields).
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	if h.users == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_USERS", "user store not configured")
		return
	}
	authUser, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	// Users can update themselves (limited fields), admins can update anyone
	if !authUser.IsAdmin() && authUser.ID != id {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "can only update your own profile")
		return
	}
	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	// Non-admins can only update name and password
	if !authUser.IsAdmin() {
		if req.Tier != "" || req.Role != "" || req.Groups != nil || req.Disabled != nil {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "only admins can change tier/role/groups/disabled")
			return
		}
	}
	if req.Name != "" {
		if err := h.users.SetName(id, req.Name); err != nil {
			writeError(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error())
			return
		}
	}
	if req.Password != "" {
		if err := h.users.SetPassword(id, req.Password); err != nil {
			writeError(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error())
			return
		}
	}
	if req.Tier != "" && authUser.IsAdmin() {
		if err := h.users.SetTier(id, req.Tier); err != nil {
			writeError(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error())
			return
		}
	}
	if req.Role != "" && authUser.IsAdmin() {
		if err := h.users.SetRole(id, req.Role); err != nil {
			writeError(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error())
			return
		}
	}
	if req.Groups != nil && authUser.IsAdmin() {
		if err := h.users.SetGroups(id, req.Groups); err != nil {
			writeError(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error())
			return
		}
	}
	if req.Disabled != nil && authUser.IsAdmin() {
		if err := h.users.SetDisabled(id, *req.Disabled); err != nil {
			writeError(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error())
			return
		}
	}
	u, exists := h.users.Find(id)
	if !exists {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "user not found after update")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// DeleteUser deletes a user (admin only).
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if h.users == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_USERS", "user store not configured")
		return
	}
	u, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	if !u.IsAdmin() {
		writeError(w, http.StatusForbidden, "ADMIN_ONLY", "admin access required")
		return
	}
	id := r.PathValue("id")
	if err := h.users.Delete(id); err != nil {
		writeError(w, http.StatusBadRequest, "DELETE_FAILED", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ExportUsers exports all users as wrapped JSON (admin only).
func (h *Handler) ExportUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAuth(w, r); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="users-export.json"`)
	h.users.ExportJSON(w)
}

// ImportUsers imports users from wrapped JSON (admin only).
func (h *Handler) ImportUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAuth(w, r); !ok {
		return
	}
	imported, err := h.users.ImportJSON(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "IMPORT_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"imported": imported})
}
