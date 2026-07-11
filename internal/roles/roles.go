package roles

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"cetus-marketdata-scanner/internal/iohelp"

	_ "modernc.org/sqlite"
)

// Role represents a user role with capabilities and limits
type Role struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	Capabilities       []string          `json:"capabilities"`
	Limits             Limits            `json:"limits"`
	DefaultPermissions Permissions       `json:"default_permissions"`
	CanManageUsers     bool              `json:"can_manage_users"`
	CanManageGroups    bool              `json:"can_manage_groups"`
	BypassThrottling   bool              `json:"bypass_throttling"`
	CreatedAt          int64             `json:"created_at"`
	UpdatedAt          int64             `json:"updated_at"`
}

// Limits defines throttling and quota limits
type Limits struct {
	APICallsPerMinute  int `json:"api_calls_per_minute"`
	APICallsPerHour    int `json:"api_calls_per_hour"`
	APICallsPerDay     int `json:"api_calls_per_day"`
	MaxStudies         int `json:"max_studies"`
	MaxSavedResults    int `json:"max_saved_results"`
	MaxGroups          int `json:"max_groups"`
	MaxGroupMembers    int `json:"max_group_members"`
	ReplayDays         int `json:"replay_days"`
	MaxSymbolsPerScan  int `json:"max_symbols_per_scan"`
	ExportMaxResults   int `json:"export_max_results"`
}

// Permissions defines default Linux-style permissions
type Permissions struct {
	Owner int `json:"owner"`
	Group int `json:"group"`
	All   int `json:"all"`
}

// Store manages roles in the database
type Store struct {
	db *sql.DB
}

// NewStore creates a new role store
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Init initializes the roles table
func (s *Store) Init() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS roles (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			capabilities_json TEXT NOT NULL,
			limits_json TEXT NOT NULL,
			default_permissions_json TEXT NOT NULL,
			can_manage_users INTEGER DEFAULT 0,
			can_manage_groups INTEGER DEFAULT 0,
			bypass_throttling INTEGER DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`)
	return err
}

// Bootstrap loads roles from JSON file if table is empty
func (s *Store) Bootstrap(rolesFile string) error {
	// Check if roles exist
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM roles").Scan(&count)
	if err != nil {
		return fmt.Errorf("check roles: %w", err)
	}

	if count > 0 {
		return nil // Already bootstrapped
	}

	// Load roles from JSON
	data, err := os.ReadFile(rolesFile)
	if err != nil {
		return fmt.Errorf("read roles file: %w", err)
	}

	var roles []Role
	if err := json.Unmarshal(data, &roles); err != nil {
		return fmt.Errorf("parse roles: %w", err)
	}

	// Insert roles
	now := time.Now().Unix()
	for _, role := range roles {
		capsJSON, _ := json.Marshal(role.Capabilities)
		limitsJSON, _ := json.Marshal(role.Limits)
		permsJSON, _ := json.Marshal(role.DefaultPermissions)

		_, err := s.db.Exec(`
			INSERT INTO roles (
				id, name, description, capabilities_json, limits_json,
				default_permissions_json, can_manage_users, can_manage_groups,
				bypass_throttling, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, role.ID, role.Name, role.Description, string(capsJSON), string(limitsJSON),
			string(permsJSON), role.CanManageUsers, role.CanManageGroups,
			role.BypassThrottling, now, now)
		if err != nil {
			return fmt.Errorf("insert role %s: %w", role.ID, err)
		}
	}

	return nil
}

// Get retrieves a role by ID
func (s *Store) Get(id string) (*Role, error) {
	var role Role
	var capsJSON, limitsJSON, permsJSON string

	err := s.db.QueryRow(`
		SELECT id, name, description, capabilities_json, limits_json,
		       default_permissions_json, can_manage_users, can_manage_groups,
		       bypass_throttling, created_at, updated_at
		FROM roles WHERE id = ?
	`, id).Scan(
		&role.ID, &role.Name, &role.Description, &capsJSON, &limitsJSON,
		&permsJSON, &role.CanManageUsers, &role.CanManageGroups,
		&role.BypassThrottling, &role.CreatedAt, &role.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(capsJSON), &role.Capabilities)
	json.Unmarshal([]byte(limitsJSON), &role.Limits)
	json.Unmarshal([]byte(permsJSON), &role.DefaultPermissions)

	return &role, nil
}

// List retrieves all roles
func (s *Store) List() ([]Role, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, capabilities_json, limits_json,
		       default_permissions_json, can_manage_users, can_manage_groups,
		       bypass_throttling, created_at, updated_at
		FROM roles ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var role Role
		var capsJSON, limitsJSON, permsJSON string

		if err := rows.Scan(
			&role.ID, &role.Name, &role.Description, &capsJSON, &limitsJSON,
			&permsJSON, &role.CanManageUsers, &role.CanManageGroups,
			&role.BypassThrottling, &role.CreatedAt, &role.UpdatedAt,
		); err != nil {
			return nil, err
		}

		json.Unmarshal([]byte(capsJSON), &role.Capabilities)
		json.Unmarshal([]byte(limitsJSON), &role.Limits)
		json.Unmarshal([]byte(permsJSON), &role.DefaultPermissions)

		roles = append(roles, role)
	}

	return roles, nil
}

// Create creates a new role
func (s *Store) Create(role Role) error {
	now := time.Now().Unix()
	role.CreatedAt = now
	role.UpdatedAt = now

	capsJSON, _ := json.Marshal(role.Capabilities)
	limitsJSON, _ := json.Marshal(role.Limits)
	permsJSON, _ := json.Marshal(role.DefaultPermissions)

	_, err := s.db.Exec(`
		INSERT INTO roles (
			id, name, description, capabilities_json, limits_json,
			default_permissions_json, can_manage_users, can_manage_groups,
			bypass_throttling, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, role.ID, role.Name, role.Description, string(capsJSON), string(limitsJSON),
		string(permsJSON), role.CanManageUsers, role.CanManageGroups,
		role.BypassThrottling, role.CreatedAt, role.UpdatedAt)

	return err
}

// Update updates an existing role
func (s *Store) Update(role Role) error {
	role.UpdatedAt = time.Now().Unix()

	capsJSON, _ := json.Marshal(role.Capabilities)
	limitsJSON, _ := json.Marshal(role.Limits)
	permsJSON, _ := json.Marshal(role.DefaultPermissions)

	_, err := s.db.Exec(`
		UPDATE roles SET
			name = ?, description = ?, capabilities_json = ?, limits_json = ?,
			default_permissions_json = ?, can_manage_users = ?, can_manage_groups = ?,
			bypass_throttling = ?, updated_at = ?
		WHERE id = ?
	`, role.Name, role.Description, string(capsJSON), string(limitsJSON),
		string(permsJSON), role.CanManageUsers, role.CanManageGroups,
		role.BypassThrottling, role.UpdatedAt, role.ID)

	return err
}

// Delete deletes a role
func (s *Store) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM roles WHERE id = ?", id)
	return err
}

// HasCapability checks if a role has a specific capability
func (s *Store) HasCapability(roleID, capability string) (bool, error) {
	role, err := s.Get(roleID)
	if err != nil {
		return false, err
	}
	if role == nil {
		return false, nil
	}

	for _, cap := range role.Capabilities {
		if cap == capability {
			return true, nil
		}
	}
	return false, nil
}

// ExportJSON writes all roles as a wrapped JSON object.
func (s *Store) ExportJSON(w io.Writer) error {
	roles, err := s.List()
	if err != nil {
		return err
	}
	return iohelp.ExportJSON(w, "roles", roles, len(roles))
}

// ImportJSON imports roles from a wrapped JSON object.
func (s *Store) ImportJSON(r io.Reader) (int, error) {
	exp, err := iohelp.ImportJSON(r)
	if err != nil {
		return 0, err
	}
	data, _ := json.Marshal(exp.Items)
	var roles []Role
	if err := json.Unmarshal(data, &roles); err != nil {
		return 0, fmt.Errorf("parse roles: %w", err)
	}
	imported := 0
	for _, role := range roles {
		if err := s.Create(role); err != nil {
			continue
		}
		imported++
	}
	return imported, nil
}
