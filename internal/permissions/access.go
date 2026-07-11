package permissions

import (
	"context"
)

// AccessChecker defines the interface for checking permissions
type AccessChecker interface {
	// CanAccess checks if a user can perform an action on a resource
	CanAccess(ctx context.Context, userID string, resourceID int64, action int) (bool, error)
	
	// IsAdmin checks if a user has admin privileges
	IsAdmin(ctx context.Context, userID string) (bool, error)
	
	// IsOwner checks if a user owns a resource
	IsOwner(ctx context.Context, userID string, resourceID int64) (bool, error)
	
	// IsGroupMember checks if a user is a member of a group
	IsGroupMember(ctx context.Context, userID string, groupID string) (bool, error)
	
	// GetResourcePermissions gets the permission bits for a resource
	GetResourcePermissions(ctx context.Context, resourceID int64) (owner, group, all int, groupID string, err error)
	
	// GetACLPermission gets the ACL permission for a user on a resource
	GetACLPermission(ctx context.Context, userID string, resourceID int64) (int, bool, error)
}

// Checker implements permission checking logic
type Checker struct {
	access AccessChecker
}

// NewChecker creates a new permission checker
func NewChecker(access AccessChecker) *Checker {
	return &Checker{access: access}
}

// CanAccess checks if a user can perform an action on a resource
// Priority: Admin > ACL > Owner > Group > All
func (c *Checker) CanAccess(ctx context.Context, userID string, resourceID int64, action int) (bool, error) {
	// Admin bypasses all checks
	isAdmin, err := c.access.IsAdmin(ctx, userID)
	if err != nil {
		return false, err
	}
	if isAdmin {
		return true, nil
	}

	// Check explicit ACLs first (highest priority after admin)
	aclPerm, hasACL, err := c.access.GetACLPermission(ctx, userID, resourceID)
	if err != nil {
		return false, err
	}
	if hasACL && HasPermission(aclPerm, action) {
		return true, nil
	}

	// Check if user is owner
	isOwner, err := c.access.IsOwner(ctx, userID, resourceID)
	if err != nil {
		return false, err
	}
	if isOwner {
		ownerPerm, _, _, _, err := c.access.GetResourcePermissions(ctx, resourceID)
		if err != nil {
			return false, err
		}
		return HasPermission(ownerPerm, action), nil
	}

	// Get resource permissions
	_, groupPerm, allPerm, groupID, err := c.access.GetResourcePermissions(ctx, resourceID)
	if err != nil {
		return false, err
	}

	// Check group permissions
	if groupID != "" {
		isMember, err := c.access.IsGroupMember(ctx, userID, groupID)
		if err != nil {
			return false, err
		}
		if isMember && HasPermission(groupPerm, action) {
			return true, nil
		}
	}

	// Check all (public) permissions
	return HasPermission(allPerm, action), nil
}

// CanRead is a convenience method for checking read access
func (c *Checker) CanRead(ctx context.Context, userID string, resourceID int64) (bool, error) {
	return c.CanAccess(ctx, userID, resourceID, ActionRead)
}

// CanWrite is a convenience method for checking write access
func (c *Checker) CanWrite(ctx context.Context, userID string, resourceID int64) (bool, error) {
	return c.CanAccess(ctx, userID, resourceID, ActionWrite)
}

// CanDelete is a convenience method for checking delete access
func (c *Checker) CanDelete(ctx context.Context, userID string, resourceID int64) (bool, error) {
	return c.CanAccess(ctx, userID, resourceID, ActionDelete)
}
