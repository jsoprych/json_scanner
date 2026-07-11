package api

import (
	"encoding/json"
	"net/http"

	"cetus-marketdata-scanner/internal/roles"
)

// ListRoles returns all available roles
func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAuth(w, r); !ok {
		return
	}

	roles, err := h.roles.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"roles": roles,
		"count": len(roles),
	})
}

// GetRole returns a specific role
func (h *Handler) GetRole(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAuth(w, r); !ok {
		return
	}

	id := r.PathValue("id")
	role, err := h.roles.Get(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}
	if role == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "role not found")
		return
	}

	writeJSON(w, http.StatusOK, role)
}

// CreateRole creates a new role (admin only)
func (h *Handler) CreateRole(w http.ResponseWriter, r *http.Request) {
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

	var role roles.Role
	if err := json.NewDecoder(r.Body).Decode(&role); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	if role.ID == "" || role.Name == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "id and name are required")
		return
	}

	if err := h.roles.Create(role); err != nil {
		writeError(w, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, role)
}

// UpdateRole updates an existing role (admin only)
func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
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

	id := r.PathValue("id")

	var role roles.Role
	if err := json.NewDecoder(r.Body).Decode(&role); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	role.ID = id
	if err := h.roles.Update(role); err != nil {
		writeError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, role)
}

// DeleteRole deletes a role (admin only)
func (h *Handler) DeleteRole(w http.ResponseWriter, r *http.Request) {
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

	id := r.PathValue("id")

	// Prevent deletion of default roles
	if id == "admin" || id == "user" || id == "guest" {
		writeError(w, http.StatusBadRequest, "PROTECTED_ROLE", "cannot delete default role")
		return
	}

	if err := h.roles.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ExportRoles exports all roles as wrapped JSON.
func (h *Handler) ExportRoles(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAuth(w, r); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="roles-export.json"`)
	h.roles.ExportJSON(w)
}

// ImportRoles imports roles from wrapped JSON (admin only).
func (h *Handler) ImportRoles(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAuth(w, r); !ok {
		return
	}
	imported, err := h.roles.ImportJSON(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "IMPORT_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"imported": imported})
}
