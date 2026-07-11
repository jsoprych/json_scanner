package groups

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Group represents a user group
type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	OwnerID     string `json:"owner_id"`
	CreatedAt   int64  `json:"created_at"`
}

// Member represents a group member
type Member struct {
	GroupID  string `json:"group_id"`
	UserID   string `json:"user_id"`
	Role     string `json:"role"` // "member" or "leader"
	JoinedAt int64  `json:"joined_at"`
}

// Store manages groups and membership
type Store struct {
	db *sql.DB
}

// NewStore creates a new group store
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Create creates a new group
func (s *Store) Create(ctx context.Context, group Group) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO groups (id, name, description, owner_id, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, group.ID, group.Name, group.Description, group.OwnerID, group.CreatedAt)
	if err != nil {
		return fmt.Errorf("create group: %w", err)
	}
	return nil
}

// Get retrieves a group by ID
func (s *Store) Get(ctx context.Context, id string) (*Group, error) {
	var group Group
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, owner_id, created_at
		FROM groups WHERE id = ?
	`, id).Scan(&group.ID, &group.Name, &group.Description, &group.OwnerID, &group.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get group: %w", err)
	}
	return &group, nil
}

// List retrieves all groups
func (s *Store) List(ctx context.Context) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, owner_id, created_at
		FROM groups ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var group Group
		if err := rows.Scan(&group.ID, &group.Name, &group.Description, &group.OwnerID, &group.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

// ListByOwner retrieves groups owned by a user
func (s *Store) ListByOwner(ctx context.Context, ownerID string) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, owner_id, created_at
		FROM groups WHERE owner_id = ? ORDER BY name
	`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list groups by owner: %w", err)
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var group Group
		if err := rows.Scan(&group.ID, &group.Name, &group.Description, &group.OwnerID, &group.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

// Update updates a group
func (s *Store) Update(ctx context.Context, group Group) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE groups SET name = ?, description = ?
		WHERE id = ?
	`, group.Name, group.Description, group.ID)
	if err != nil {
		return fmt.Errorf("update group: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("group not found: %s", group.ID)
	}
	return nil
}

// Delete deletes a group
func (s *Store) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("group not found: %s", id)
	}
	return nil
}

// AddMember adds a user to a group
func (s *Store) AddMember(ctx context.Context, groupID, userID, role string) error {
	if role == "" {
		role = "member"
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO group_members (group_id, user_id, role, joined_at)
		VALUES (?, ?, ?, ?)
	`, groupID, userID, role, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	return nil
}

// RemoveMember removes a user from a group
func (s *Store) RemoveMember(ctx context.Context, groupID, userID string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM group_members WHERE group_id = ? AND user_id = ?
	`, groupID, userID)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("member not found")
	}
	return nil
}

// GetMember retrieves a group member
func (s *Store) GetMember(ctx context.Context, groupID, userID string) (*Member, error) {
	var member Member
	err := s.db.QueryRowContext(ctx, `
		SELECT group_id, user_id, role, joined_at
		FROM group_members WHERE group_id = ? AND user_id = ?
	`, groupID, userID).Scan(&member.GroupID, &member.UserID, &member.Role, &member.JoinedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get member: %w", err)
	}
	return &member, nil
}

// ListMembers retrieves all members of a group
func (s *Store) ListMembers(ctx context.Context, groupID string) ([]Member, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT group_id, user_id, role, joined_at
		FROM group_members WHERE group_id = ? ORDER BY joined_at
	`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var members []Member
	for rows.Next() {
		var member Member
		if err := rows.Scan(&member.GroupID, &member.UserID, &member.Role, &member.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

// ListUserGroups retrieves all groups a user belongs to
func (s *Store) ListUserGroups(ctx context.Context, userID string) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id, g.name, g.description, g.owner_id, g.created_at
		FROM groups g
		INNER JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.user_id = ?
		ORDER BY g.name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user groups: %w", err)
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var group Group
		if err := rows.Scan(&group.ID, &group.Name, &group.Description, &group.OwnerID, &group.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

// IsMember checks if a user is a member of a group
func (s *Store) IsMember(ctx context.Context, groupID, userID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM group_members
		WHERE group_id = ? AND user_id = ?
	`, groupID, userID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check membership: %w", err)
	}
	return count > 0, nil
}

// UpdateMemberRole updates a member's role
func (s *Store) UpdateMemberRole(ctx context.Context, groupID, userID, role string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE group_members SET role = ?
		WHERE group_id = ? AND user_id = ?
	`, role, groupID, userID)
	if err != nil {
		return fmt.Errorf("update member role: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("member not found")
	}
	return nil
}
