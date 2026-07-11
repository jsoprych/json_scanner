package results

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Result represents a saved scan result
type Result struct {
	ID           int64   `json:"id"`
	UserID       string  `json:"user_id"`
	StudyID      string  `json:"study_id"`
	SnapshotDate int64   `json:"snapshot_date"`
	Results      []Match `json:"results"`
	CreatedAt    int64   `json:"created_at"`
	Name         string  `json:"name,omitempty"`
	Notes        string  `json:"notes,omitempty"`
	
	// Linux-style permissions (private by default)
	PermOwner int    `json:"perm_owner"`
	PermGroup int    `json:"perm_group"`
	PermAll   int    `json:"perm_all"`
	GroupID   string `json:"group_id,omitempty"`
}

// Match represents a single scan match
type Match struct {
	Symbol    string  `json:"symbol"`
	Close     float64 `json:"close"`
	RSI14     float64 `json:"rsi14"`
	Ret3m     float64 `json:"ret_3m"`
	DollarVol float64 `json:"dollar_vol"`
}

// ACL represents an access control list entry
type ACL struct {
	ResultID  int64  `json:"result_id"`
	UserID    string `json:"user_id"`
	Permission int   `json:"permission"`
	GrantedAt int64  `json:"granted_at"`
	GrantedBy string `json:"granted_by"`
}

// Store manages saved results
type Store struct {
	db *sql.DB
}

// NewStore creates a new results store
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Save saves a new result
func (s *Store) Save(ctx context.Context, result Result) (int64, error) {
	resultsJSON, err := json.Marshal(result.Results)
	if err != nil {
		return 0, fmt.Errorf("marshal results: %w", err)
	}
	
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO saved_results (
			user_id, study_id, snapshot_date, results_json, created_at,
			name, notes, perm_owner, perm_group, perm_all, group_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, 
		result.UserID, result.StudyID, result.SnapshotDate, string(resultsJSON), result.CreatedAt,
		result.Name, result.Notes, result.PermOwner, result.PermGroup, result.PermAll, result.GroupID,
	)
	if err != nil {
		return 0, fmt.Errorf("save result: %w", err)
	}
	
	return res.LastInsertId()
}

// Get retrieves a result by ID
func (s *Store) Get(ctx context.Context, id int64) (*Result, error) {
	var result Result
	var resultsJSON string
	
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, study_id, snapshot_date, results_json, created_at,
		       name, notes, perm_owner, perm_group, perm_all, group_id
		FROM saved_results WHERE id = ?
	`, id).Scan(
		&result.ID, &result.UserID, &result.StudyID, &result.SnapshotDate,
		&resultsJSON, &result.CreatedAt, &result.Name, &result.Notes,
		&result.PermOwner, &result.PermGroup, &result.PermAll, &result.GroupID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get result: %w", err)
	}
	
	if err := json.Unmarshal([]byte(resultsJSON), &result.Results); err != nil {
		return nil, fmt.Errorf("unmarshal results: %w", err)
	}
	
	return &result, nil
}

// List retrieves results for a user
func (s *Store) List(ctx context.Context, userID string, limit, offset int) ([]Result, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, study_id, snapshot_date, results_json, created_at,
		       name, notes, perm_owner, perm_group, perm_all, group_id
		FROM saved_results 
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list results: %w", err)
	}
	defer rows.Close()
	
	var results []Result
	for rows.Next() {
		var result Result
		var resultsJSON string
		
		if err := rows.Scan(
			&result.ID, &result.UserID, &result.StudyID, &result.SnapshotDate,
			&resultsJSON, &result.CreatedAt, &result.Name, &result.Notes,
			&result.PermOwner, &result.PermGroup, &result.PermAll, &result.GroupID,
		); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		
		if err := json.Unmarshal([]byte(resultsJSON), &result.Results); err != nil {
			return nil, fmt.Errorf("unmarshal results: %w", err)
		}
		
		results = append(results, result)
	}
	
	return results, rows.Err()
}

// ListAccessible retrieves results accessible to a user (owner, group, or public)
func (s *Store) ListAccessible(ctx context.Context, userID string, limit, offset int) ([]Result, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT sr.id, sr.user_id, sr.study_id, sr.snapshot_date, 
		       sr.results_json, sr.created_at, sr.name, sr.notes,
		       sr.perm_owner, sr.perm_group, sr.perm_all, sr.group_id
		FROM saved_results sr
		LEFT JOIN group_members gm ON sr.group_id = gm.group_id AND gm.user_id = ?
		WHERE sr.user_id = ?  -- owned by user
		   OR (sr.perm_all & 4) > 0  -- public read
		   OR (gm.user_id IS NOT NULL AND (sr.perm_group & 4) > 0)  -- group read
		ORDER BY sr.created_at DESC
		LIMIT ? OFFSET ?
	`, userID, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list accessible results: %w", err)
	}
	defer rows.Close()
	
	var results []Result
	for rows.Next() {
		var result Result
		var resultsJSON string
		
		if err := rows.Scan(
			&result.ID, &result.UserID, &result.StudyID, &result.SnapshotDate,
			&resultsJSON, &result.CreatedAt, &result.Name, &result.Notes,
			&result.PermOwner, &result.PermGroup, &result.PermAll, &result.GroupID,
		); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		
		if err := json.Unmarshal([]byte(resultsJSON), &result.Results); err != nil {
			return nil, fmt.Errorf("unmarshal results: %w", err)
		}
		
		results = append(results, result)
	}
	
	return results, rows.Err()
}

// Update updates a result
func (s *Store) Update(ctx context.Context, result Result) error {
	resultsJSON, err := json.Marshal(result.Results)
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}
	
	res, err := s.db.ExecContext(ctx, `
		UPDATE saved_results SET
			study_id = ?, snapshot_date = ?, results_json = ?,
			name = ?, notes = ?, perm_owner = ?, perm_group = ?, 
			perm_all = ?, group_id = ?
		WHERE id = ?
	`, 
		result.StudyID, result.SnapshotDate, string(resultsJSON),
		result.Name, result.Notes, result.PermOwner, result.PermGroup,
		result.PermAll, result.GroupID, result.ID,
	)
	if err != nil {
		return fmt.Errorf("update result: %w", err)
	}
	
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("result not found: %d", result.ID)
	}
	
	return nil
}

// Delete deletes a result
func (s *Store) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM saved_results WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete result: %w", err)
	}
	
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("result not found: %d", id)
	}
	
	return nil
}

// UpdatePermissions updates result permissions (chmod)
func (s *Store) UpdatePermissions(ctx context.Context, id int64, owner, group, all int, groupID string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE saved_results SET
			perm_owner = ?, perm_group = ?, perm_all = ?, group_id = ?
		WHERE id = ?
	`, owner, group, all, groupID, id)
	if err != nil {
		return fmt.Errorf("update permissions: %w", err)
	}
	
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("result not found: %d", id)
	}
	
	return nil
}

// ChangeOwner changes result ownership (chown)
func (s *Store) ChangeOwner(ctx context.Context, id int64, userID, groupID string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE saved_results SET user_id = ?, group_id = ?
		WHERE id = ?
	`, userID, groupID, id)
	if err != nil {
		return fmt.Errorf("change owner: %w", err)
	}
	
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("result not found: %d", id)
	}
	
	return nil
}

// AddACL adds an ACL entry (setfacl)
func (s *Store) AddACL(ctx context.Context, acl ACL) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO result_acls (result_id, user_id, permission, granted_at, granted_by)
		VALUES (?, ?, ?, ?, ?)
	`, acl.ResultID, acl.UserID, acl.Permission, acl.GrantedAt, acl.GrantedBy)
	if err != nil {
		return fmt.Errorf("add acl: %w", err)
	}
	return nil
}

// RemoveACL removes an ACL entry (setfacl -x)
func (s *Store) RemoveACL(ctx context.Context, resultID int64, userID string) error {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM result_acls WHERE result_id = ? AND user_id = ?
	`, resultID, userID)
	if err != nil {
		return fmt.Errorf("remove acl: %w", err)
	}
	
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("acl not found")
	}
	
	return nil
}

// GetACL retrieves an ACL entry
func (s *Store) GetACL(ctx context.Context, resultID int64, userID string) (*ACL, error) {
	var acl ACL
	err := s.db.QueryRowContext(ctx, `
		SELECT result_id, user_id, permission, granted_at, granted_by
		FROM result_acls WHERE result_id = ? AND user_id = ?
	`, resultID, userID).Scan(&acl.ResultID, &acl.UserID, &acl.Permission, &acl.GrantedAt, &acl.GrantedBy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get acl: %w", err)
	}
	return &acl, nil
}

// ListACLs retrieves all ACLs for a result
func (s *Store) ListACLs(ctx context.Context, resultID int64) ([]ACL, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT result_id, user_id, permission, granted_at, granted_by
		FROM result_acls WHERE result_id = ?
		ORDER BY granted_at
	`, resultID)
	if err != nil {
		return nil, fmt.Errorf("list acls: %w", err)
	}
	defer rows.Close()
	
	var acls []ACL
	for rows.Next() {
		var acl ACL
		if err := rows.Scan(&acl.ResultID, &acl.UserID, &acl.Permission, &acl.GrantedAt, &acl.GrantedBy); err != nil {
			return nil, fmt.Errorf("scan acl: %w", err)
		}
		acls = append(acls, acl)
	}
	
	return acls, rows.Err()
}

// ExportData represents export format
type ExportData struct {
	Version   string   `json:"version"`
	ExportedAt int64   `json:"exported_at"`
	UserID    string   `json:"user_id"`
	Results   []Result `json:"results"`
}

// Export exports results as JSON
func (s *Store) Export(ctx context.Context, userID string) (*ExportData, error) {
	results, err := s.List(ctx, userID, 0, 0) // Get all
	if err != nil {
		return nil, err
	}
	
	// Load ACLs for each result
	for i := range results {
		acls, err := s.ListACLs(ctx, results[i].ID)
		if err != nil {
			return nil, err
		}
		// Store ACLs in result (we'll need to add this field)
		_ = acls // TODO: Add ACLs to Result struct if needed
	}
	
	return &ExportData{
		Version:    "1.0",
		ExportedAt: time.Now().Unix(),
		UserID:     userID,
		Results:    results,
	}, nil
}

// Import imports results from JSON
func (s *Store) Import(ctx context.Context, data *ExportData) (int, error) {
	imported := 0
	
	for _, result := range data.Results {
		// Check if result already exists (by study_id and snapshot_date)
		existing, err := s.GetByStudyAndDate(ctx, result.UserID, result.StudyID, result.SnapshotDate)
		if err != nil {
			return imported, err
		}
		
		if existing != nil {
			// Skip duplicate
			continue
		}
		
		// Import result
		_, err = s.Save(ctx, result)
		if err != nil {
			return imported, err
		}
		imported++
	}
	
	return imported, nil
}

// GetByStudyAndDate retrieves a result by study ID and snapshot date
func (s *Store) GetByStudyAndDate(ctx context.Context, userID, studyID string, snapshotDate int64) (*Result, error) {
	var result Result
	var resultsJSON string
	
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, study_id, snapshot_date, results_json, created_at,
		       name, notes, perm_owner, perm_group, perm_all, group_id
		FROM saved_results 
		WHERE user_id = ? AND study_id = ? AND snapshot_date = ?
	`, userID, studyID, snapshotDate).Scan(
		&result.ID, &result.UserID, &result.StudyID, &result.SnapshotDate,
		&resultsJSON, &result.CreatedAt, &result.Name, &result.Notes,
		&result.PermOwner, &result.PermGroup, &result.PermAll, &result.GroupID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get result by study and date: %w", err)
	}
	
	if err := json.Unmarshal([]byte(resultsJSON), &result.Results); err != nil {
		return nil, fmt.Errorf("unmarshal results: %w", err)
	}
	
	return &result, nil
}
