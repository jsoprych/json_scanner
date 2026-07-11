package api

import (
	"encoding/json"
	"net/http"
	"time"

	"cetus-marketdata-scanner/internal/groups"
)

// CreateGroup creates a new group
func (h *Handler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	var req struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	if req.ID == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "id and name are required")
		return
	}

	group := groups.Group{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     user.ID,
		CreatedAt:   time.Now().Unix(),
	}

	if err := h.groups.Create(r.Context(), group); err != nil {
		writeError(w, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	// Add owner as leader
	if err := h.groups.AddMember(r.Context(), group.ID, user.ID, "leader"); err != nil {
		writeError(w, http.StatusInternalServerError, "ADD_MEMBER_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, group)
}

// GetGroup retrieves a group by ID
func (h *Handler) GetGroup(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAuth(w, r); !ok {
		return
	}

	id := r.PathValue("id")
	group, err := h.groups.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}
	if group == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "group not found")
		return
	}

	writeJSON(w, http.StatusOK, group)
}

// ListGroups retrieves all groups
func (h *Handler) ListGroups(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAuth(w, r); !ok {
		return
	}

	groups, err := h.groups.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"groups": groups,
		"count":  len(groups),
	})
}

// ListUserGroups retrieves groups for a user
func (h *Handler) ListUserGroups(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	userID := r.PathValue("user_id")
	if userID == "" {
		userID = user.ID
	}

	// Non-admins can only list their own groups
	if userID != user.ID && !user.IsAdmin() {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "can only list your own groups")
		return
	}

	groups, err := h.groups.ListUserGroups(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"groups": groups,
		"count":  len(groups),
	})
}

// DeleteGroup deletes a group
func (h *Handler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")
	group, err := h.groups.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}
	if group == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "group not found")
		return
	}

	// Only owner or admin can delete
	if group.OwnerID != user.ID && !user.IsAdmin() {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "only group owner or admin can delete")
		return
	}

	if err := h.groups.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AddGroupMember adds a user to a group
func (h *Handler) AddGroupMember(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	groupID := r.PathValue("id")
	group, err := h.groups.Get(r.Context(), groupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}
	if group == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "group not found")
		return
	}

	// Check if user is group leader or admin
	member, err := h.groups.GetMember(r.Context(), groupID, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_MEMBER_FAILED", err.Error())
		return
	}
	if (member == nil || member.Role != "leader") && !user.IsAdmin() {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "only group leader or admin can add members")
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "user_id is required")
		return
	}

	if req.Role == "" {
		req.Role = "member"
	}

	if err := h.groups.AddMember(r.Context(), groupID, req.UserID, req.Role); err != nil {
		writeError(w, http.StatusInternalServerError, "ADD_MEMBER_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"status": "added",
	})
}

// RemoveGroupMember removes a user from a group
func (h *Handler) RemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	groupID := r.PathValue("id")
	userID := r.PathValue("user_id")

	group, err := h.groups.Get(r.Context(), groupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}
	if group == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "group not found")
		return
	}

	// Check if user is group leader or admin
	member, err := h.groups.GetMember(r.Context(), groupID, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_MEMBER_FAILED", err.Error())
		return
	}
	if (member == nil || member.Role != "leader") && !user.IsAdmin() {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "only group leader or admin can remove members")
		return
	}

	if err := h.groups.RemoveMember(r.Context(), groupID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "REMOVE_MEMBER_FAILED", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListGroupMembers retrieves all members of a group
func (h *Handler) ListGroupMembers(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAuth(w, r); !ok {
		return
	}

	groupID := r.PathValue("id")
	members, err := h.groups.ListMembers(r.Context(), groupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"members": members,
		"count":   len(members),
	})
}

// ExportGroups exports all groups as wrapped JSON.
func (h *Handler) ExportGroups(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAuth(w, r); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="groups-export.json"`)
	h.groups.ExportJSON(r.Context(), w)
}

// ImportGroups imports groups from wrapped JSON.
func (h *Handler) ImportGroups(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAuth(w, r); !ok {
		return
	}
	imported, err := h.groups.ImportJSON(r.Context(), r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "IMPORT_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"imported": imported})
}
