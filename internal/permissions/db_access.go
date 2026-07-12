package permissions

import (
	"context"
	"database/sql"
	"cetus-marketdata-scanner/internal/dblog"
	"fmt"
)

// DBAccessChecker implements AccessChecker using database stores
type DBAccessChecker struct {
	db *dblog.DB
}

// NewDBAccessChecker creates a new database-backed access checker
func NewDBAccessChecker(db *dblog.DB) *DBAccessChecker {
	return &DBAccessChecker{db: db}
}

// IsAdmin checks if a user has admin privileges
func (c *DBAccessChecker) IsAdmin(ctx context.Context, userID string) (bool, error) {
	var roleID string
	err := c.db.QueryRowContext(ctx, `
		SELECT role_id FROM users WHERE id = ?
	`, userID).Scan(&roleID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check admin: %w", err)
	}
	return roleID == "admin", nil
}

// IsOwner checks if a user owns a resource
func (c *DBAccessChecker) IsOwner(ctx context.Context, userID string, resourceID int64) (bool, error) {
	var ownerID string
	err := c.db.QueryRowContext(ctx, `
		SELECT user_id FROM saved_results WHERE id = ?
	`, resourceID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check owner: %w", err)
	}
	return ownerID == userID, nil
}

// IsGroupMember checks if a user is a member of a group
func (c *DBAccessChecker) IsGroupMember(ctx context.Context, userID string, groupID string) (bool, error) {
	var count int
	err := c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM group_members 
		WHERE group_id = ? AND user_id = ?
	`, groupID, userID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check group member: %w", err)
	}
	return count > 0, nil
}

// GetResourcePermissions gets the permission bits for a resource
func (c *DBAccessChecker) GetResourcePermissions(ctx context.Context, resourceID int64) (owner, group, all int, groupID string, err error) {
	err = c.db.QueryRowContext(ctx, `
		SELECT perm_owner, perm_group, perm_all, group_id 
		FROM saved_results WHERE id = ?
	`, resourceID).Scan(&owner, &group, &all, &groupID)
	if err == sql.ErrNoRows {
		return 0, 0, 0, "", fmt.Errorf("resource not found")
	}
	if err != nil {
		return 0, 0, 0, "", fmt.Errorf("get permissions: %w", err)
	}
	return owner, group, all, groupID, nil
}

// GetACLPermission gets the ACL permission for a user on a resource
func (c *DBAccessChecker) GetACLPermission(ctx context.Context, userID string, resourceID int64) (int, bool, error) {
	var permission int
	err := c.db.QueryRowContext(ctx, `
		SELECT permission FROM result_acls 
		WHERE result_id = ? AND user_id = ?
	`, resourceID, userID).Scan(&permission)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("get acl permission: %w", err)
	}
	return permission, true, nil
}

// CanAccess checks if a user can perform an action on a resource
func (c *DBAccessChecker) CanAccess(ctx context.Context, userID string, resourceID int64, action int) (bool, error) {
	checker := NewChecker(c)
	return checker.CanAccess(ctx, userID, resourceID, action)
}
