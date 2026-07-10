package api

import (
	"encoding/json"
	"net/http"

	"cetus-marketdata-scanner/internal/study"
)

// ListStudies returns all studies.
func (h *Handler) ListStudies(w http.ResponseWriter, r *http.Request) {
	if h.studies == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_STUDIES", "studies not configured")
		return
	}
	writeJSON(w, http.StatusOK, h.studies.All())
}

// CreateStudy creates a new study.
func (h *Handler) CreateStudy(w http.ResponseWriter, r *http.Request) {
	if h.studies == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_STUDIES", "studies not configured")
		return
	}
	var st study.Study
	if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if err := h.studies.Upsert(st); err != nil {
		writeError(w, http.StatusBadRequest, "CREATE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, st)
}

// GetStudy returns a study by key.
func (h *Handler) GetStudy(w http.ResponseWriter, r *http.Request) {
	if h.studies == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_STUDIES", "studies not configured")
		return
	}
	key := r.PathValue("id")
	st, ok := h.studies.Get(key)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "study not found")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// UpdateStudy updates an existing study.
func (h *Handler) UpdateStudy(w http.ResponseWriter, r *http.Request) {
	if h.studies == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_STUDIES", "studies not configured")
		return
	}
	key := r.PathValue("id")
	if _, ok := h.studies.Get(key); !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "study not found")
		return
	}
	var st study.Study
	if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	st.Key = key
	if err := h.studies.Upsert(st); err != nil {
		writeError(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// DeleteStudy deletes a study by key.
func (h *Handler) DeleteStudy(w http.ResponseWriter, r *http.Request) {
	if h.studies == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_STUDIES", "studies not configured")
		return
	}
	key := r.PathValue("id")
	if err := h.studies.Delete(key); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
