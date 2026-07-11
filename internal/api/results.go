package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"cetus-marketdata-scanner/internal/permissions"
	"cetus-marketdata-scanner/internal/results"
)

// SaveResult saves a new scan result
func (h *Handler) SaveResult(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	var req struct {
		StudyID      string          `json:"study_id"`
		SnapshotDate int64           `json:"snapshot_date"`
		Results      []results.Match `json:"results"`
		Name         string          `json:"name,omitempty"`
		Notes        string          `json:"notes,omitempty"`
		PermOwner    int             `json:"perm_owner,omitempty"`
		PermGroup    int             `json:"perm_group,omitempty"`
		PermAll      int             `json:"perm_all,omitempty"`
		GroupID      string          `json:"group_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	if req.StudyID == "" || req.SnapshotDate == 0 {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "study_id and snapshot_date are required")
		return
	}

	// Set default permissions (private by default)
	if req.PermOwner == 0 {
		req.PermOwner = permissions.PermFull // 7
	}
	if !permissions.ValidPermission(req.PermOwner) ||
		!permissions.ValidPermission(req.PermGroup) ||
		!permissions.ValidPermission(req.PermAll) {
		writeError(w, http.StatusBadRequest, "INVALID_PERMISSIONS", "permissions must be 0-7")
		return
	}

	result := results.Result{
		UserID:       user.ID,
		StudyID:      req.StudyID,
		SnapshotDate: req.SnapshotDate,
		Results:      req.Results,
		CreatedAt:    time.Now().Unix(),
		Name:         req.Name,
		Notes:        req.Notes,
		PermOwner:    req.PermOwner,
		PermGroup:    req.PermGroup,
		PermAll:      req.PermAll,
		GroupID:      req.GroupID,
	}

	id, err := h.results.Save(r.Context(), result)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SAVE_FAILED", err.Error())
		return
	}

	result.ID = id
	writeJSON(w, http.StatusCreated, result)
}

// GetResult retrieves a result by ID
func (h *Handler) GetResult(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "id must be an integer")
		return
	}

	result, err := h.results.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}
	if result == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "result not found")
		return
	}

	// Check read permission
	canRead, err := h.permChecker.CanRead(r.Context(), user.ID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PERMISSION_CHECK_FAILED", err.Error())
		return
	}
	if !canRead {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "no read permission")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ListResults retrieves results for the current user
func (h *Handler) ListResults(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	results, err := h.results.List(r.Context(), user.ID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"results": results,
		"count":   len(results),
		"limit":   limit,
		"offset":  offset,
	})
}

// ListAccessibleResults retrieves results accessible to the current user
func (h *Handler) ListAccessibleResults(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	results, err := h.results.ListAccessible(r.Context(), user.ID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"results": results,
		"count":   len(results),
		"limit":   limit,
		"offset":  offset,
	})
}

// DeleteResult deletes a result
func (h *Handler) DeleteResult(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "id must be an integer")
		return
	}

	// Check delete permission
	canDelete, err := h.permChecker.CanDelete(r.Context(), user.ID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PERMISSION_CHECK_FAILED", err.Error())
		return
	}
	if !canDelete {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "no delete permission")
		return
	}

	if err := h.results.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateResultPermissions updates result permissions (chmod)
func (h *Handler) UpdateResultPermissions(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "id must be an integer")
		return
	}

	// Check write permission
	canWrite, err := h.permChecker.CanWrite(r.Context(), user.ID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PERMISSION_CHECK_FAILED", err.Error())
		return
	}
	if !canWrite {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "no write permission")
		return
	}

	var req struct {
		PermOwner int    `json:"perm_owner"`
		PermGroup int    `json:"perm_group"`
		PermAll   int    `json:"perm_all"`
		GroupID   string `json:"group_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	if !permissions.ValidPermission(req.PermOwner) ||
		!permissions.ValidPermission(req.PermGroup) ||
		!permissions.ValidPermission(req.PermAll) {
		writeError(w, http.StatusBadRequest, "INVALID_PERMISSIONS", "permissions must be 0-7")
		return
	}

	if err := h.results.UpdatePermissions(r.Context(), id, req.PermOwner, req.PermGroup, req.PermAll, req.GroupID); err != nil {
		writeError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "updated",
	})
}

// ChangeResultOwner changes result ownership (chown)
func (h *Handler) ChangeResultOwner(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "id must be an integer")
		return
	}

	// Only owner or admin can change ownership
	result, err := h.results.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}
	if result == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "result not found")
		return
	}
	if result.UserID != user.ID && !user.IsAdmin() {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "only owner or admin can change ownership")
		return
	}

	var req struct {
		UserID  string `json:"user_id"`
		GroupID string `json:"group_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "user_id is required")
		return
	}

	if err := h.results.ChangeOwner(r.Context(), id, req.UserID, req.GroupID); err != nil {
		writeError(w, http.StatusInternalServerError, "CHANGE_OWNER_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ownership changed",
	})
}

// AddResultACL adds an ACL entry (setfacl)
func (h *Handler) AddResultACL(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "id must be an integer")
		return
	}

	// Check write permission
	canWrite, err := h.permChecker.CanWrite(r.Context(), user.ID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PERMISSION_CHECK_FAILED", err.Error())
		return
	}
	if !canWrite {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "no write permission")
		return
	}

	var req struct {
		UserID     string `json:"user_id"`
		Permission int    `json:"permission"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "user_id is required")
		return
	}

	if !permissions.ValidPermission(req.Permission) {
		writeError(w, http.StatusBadRequest, "INVALID_PERMISSION", "permission must be 0-7")
		return
	}

	acl := results.ACL{
		ResultID:   id,
		UserID:     req.UserID,
		Permission: req.Permission,
		GrantedAt:  time.Now().Unix(),
		GrantedBy:  user.ID,
	}

	if err := h.results.AddACL(r.Context(), acl); err != nil {
		writeError(w, http.StatusInternalServerError, "ADD_ACL_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, acl)
}

// RemoveResultACL removes an ACL entry (setfacl -x)
func (h *Handler) RemoveResultACL(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "id must be an integer")
		return
	}

	userID := r.PathValue("user_id")

	// Check write permission
	canWrite, err := h.permChecker.CanWrite(r.Context(), user.ID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PERMISSION_CHECK_FAILED", err.Error())
		return
	}
	if !canWrite {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "no write permission")
		return
	}

	if err := h.results.RemoveACL(r.Context(), id, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "REMOVE_ACL_FAILED", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListResultACLs retrieves all ACLs for a result
func (h *Handler) ListResultACLs(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "id must be an integer")
		return
	}

	// Check read permission
	canRead, err := h.permChecker.CanRead(r.Context(), user.ID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PERMISSION_CHECK_FAILED", err.Error())
		return
	}
	if !canRead {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "no read permission")
		return
	}

	acls, err := h.results.ListACLs(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"acls":  acls,
		"count": len(acls),
	})
}

// ExportResults exports results as JSON
func (h *Handler) ExportResults(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	data, err := h.results.Export(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "EXPORT_FAILED", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=results-export.json")
	json.NewEncoder(w).Encode(data)
}

// ImportResults imports results from JSON
func (h *Handler) ImportResults(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	var data results.ExportData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	// Override user ID to current user
	data.UserID = user.ID

	imported, err := h.results.Import(r.Context(), &data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "IMPORT_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "imported",
		"imported": imported,
	})
}
