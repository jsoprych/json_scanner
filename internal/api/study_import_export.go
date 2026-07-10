package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"cetus-marketdata-scanner/internal/study"
	"cetus-marketdata-scanner/internal/user"
)

// ImportStudies handles POST /api/v1/studies/import
// Accepts JSONL (one study per line) or JSON array
// Returns count of imported studies and any errors
func (h *Handler) ImportStudies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authenticated user
	u, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read body: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Try to parse as JSON array first
	var studies []study.Study
	if err := json.Unmarshal(body, &studies); err != nil {
		// If that fails, try JSONL (one JSON object per line)
		studies, err = parseJSONL(body)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse studies: %v", err), http.StatusBadRequest)
			return
		}
	}

	if len(studies) == 0 {
		http.Error(w, "No studies found in import data", http.StatusBadRequest)
		return
	}

	// Validate and import each study
	var imported []study.Study
	var errors []string

	for i, s := range studies {
		// Validate the study
		if err := validateStudy(s); err != nil {
			errors = append(errors, fmt.Sprintf("Study %d (%s): %v", i+1, s.Key, err))
			continue
		}

		// Set owner to current user if not admin, or use provided owner if admin
		if !u.IsAdmin() {
			s.Owner = u.ID
		} else if s.Owner == "" {
			s.Owner = user.GlobalID
		}

		// Set defaults
		if s.Visibility == "" {
			s.Visibility = study.VisPrivate
		}
		if s.Tier == "" {
			s.Tier = user.TierFree
		}

		// Save the study
		if err := h.studies.Upsert(s); err != nil {
			errors = append(errors, fmt.Sprintf("Study %d (%s): %v", i+1, s.Key, err))
			continue
		}

		imported = append(imported, s)
	}

	// Return result
	result := map[string]interface{}{
		"imported": len(imported),
		"total":    len(studies),
	}

	if len(errors) > 0 {
		result["errors"] = errors
	}

	w.Header().Set("Content-Type", "application/json")
	if len(imported) == 0 {
		w.WriteHeader(http.StatusBadRequest)
	} else if len(errors) > 0 {
		w.WriteHeader(http.StatusMultiStatus) // 207 Partial Content
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(result)
}

// ExportStudies handles GET /api/v1/studies/export
// Exports studies as JSONL (one study per line)
// Query params:
//   - owner: filter by owner (default: current user, or "all" for admins)
//   - format: "jsonl" (default) or "json" (array)
func (h *Handler) ExportStudies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authenticated user
	u, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	// Get query parameters
	ownerFilter := r.URL.Query().Get("owner")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "jsonl"
	}

	// Get all studies
	allStudies := h.studies.All()

	// Filter studies based on user role and owner filter
	var studies []study.Study
	for _, s := range allStudies {
		// Admin can export all studies
		if u.IsAdmin() {
			if ownerFilter == "" || ownerFilter == "all" || s.Owner == ownerFilter {
				studies = append(studies, s)
			}
			continue
		}

		// Regular users can only export their own studies
		if s.Owner == u.ID {
			if ownerFilter == "" || ownerFilter == u.ID {
				studies = append(studies, s)
			}
		}
	}

	if len(studies) == 0 {
		http.Error(w, "No studies found to export", http.StatusNotFound)
		return
	}

	// Export in requested format
	w.Header().Set("Content-Type", "application/json")

	if format == "json" {
		// Export as JSON array
		w.Header().Set("Content-Disposition", "attachment; filename=studies.json")
		json.NewEncoder(w).Encode(studies)
	} else {
		// Export as JSONL (default)
		w.Header().Set("Content-Disposition", "attachment; filename=studies.jsonl")
		w.WriteHeader(http.StatusOK)

		for _, s := range studies {
			if err := json.NewEncoder(w).Encode(s); err != nil {
				// Can't change status code after writing, so just log and continue
				fmt.Printf("Error encoding study %s: %v\n", s.Key, err)
			}
		}
	}
}

// parseJSONL parses JSONL format (one JSON object per line)
func parseJSONL(data []byte) ([]study.Study, error) {
	var studies []study.Study
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line size

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var s study.Study
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}

		studies = append(studies, s)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning JSONL: %w", err)
	}

	return studies, nil
}

// validateStudy validates a study before import
func validateStudy(s study.Study) error {
	// Key is required
	if s.Key == "" {
		return fmt.Errorf("key is required")
	}

	// Key must be alphanumeric with hyphens/underscores only
	for _, c := range s.Key {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return fmt.Errorf("key must contain only alphanumeric characters, hyphens, and underscores")
		}
	}

	// Where clause is required
	if s.Where == "" {
		return fmt.Errorf("where clause is required")
	}

	// Validate WHERE clause syntax
	if err := study.ValidateClause(s.Where); err != nil {
		return fmt.Errorf("invalid WHERE clause: %w", err)
	}

	// Validate ORDER BY clause if present
	if s.OrderBy != "" {
		if err := study.ValidateClause(s.OrderBy); err != nil {
			return fmt.Errorf("invalid ORDER BY clause: %w", err)
		}
	}

	// Validate visibility if present
	if s.Visibility != "" {
		switch s.Visibility {
		case study.VisPrivate, study.VisGroup, study.VisPublic:
			// Valid
		default:
			return fmt.Errorf("invalid visibility: %s (must be private, group, or public)", s.Visibility)
		}
	}

	// Validate tier if present
	if s.Tier != "" {
		switch s.Tier {
		case user.TierFree, user.TierPro:
			// Valid
		default:
			return fmt.Errorf("invalid tier: %s (must be free or pro)", s.Tier)
		}
	}

	// Validate limit if present
	if s.Limit < 0 {
		return fmt.Errorf("limit must be non-negative")
	}

	// Group is required if visibility is group
	if s.Visibility == study.VisGroup && s.Group == "" {
		return fmt.Errorf("group is required when visibility is 'group'")
	}

	return nil
}
