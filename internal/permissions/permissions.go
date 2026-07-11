package permissions

// Permission bits (Linux-style)
const (
	PermRead   = 4 // r
	PermWrite  = 2 // w
	PermDelete = 1 // x (delete)
)

// Common permission combinations
const (
	PermNone      = 0 // ---
	PermReadOnly  = 4 // r--
	PermReadWrite = 6 // rw-
	PermReadExec  = 5 // r-x
	PermFull      = 7 // rwx
)

// DefaultPermissions returns the default permission set (private by default)
func DefaultPermissions() (owner, group, all int) {
	return PermFull, PermNone, PermNone // 7, 0, 0
}

// HasPermission checks if a permission bitmask includes a specific action
func HasPermission(perms, action int) bool {
	return (perms & action) == action
}

// AddPermission adds a permission to a bitmask
func AddPermission(perms, action int) int {
	return perms | action
}

// RemovePermission removes a permission from a bitmask
func RemovePermission(perms, action int) int {
	return perms &^ action
}

// PermissionString converts a permission bitmask to a string (e.g., "rwx")
func PermissionString(perms int) string {
	result := ""
	if HasPermission(perms, PermRead) {
		result += "r"
	} else {
		result += "-"
	}
	if HasPermission(perms, PermWrite) {
		result += "w"
	} else {
		result += "-"
	}
	if HasPermission(perms, PermDelete) {
		result += "x"
	} else {
		result += "-"
	}
	return result
}

// ParsePermissionString parses a permission string (e.g., "rwx") to a bitmask
func ParsePermissionString(s string) (int, error) {
	if len(s) != 3 {
		return 0, nil
	}
	perms := 0
	if s[0] == 'r' {
		perms |= PermRead
	}
	if s[1] == 'w' {
		perms |= PermWrite
	}
	if s[2] == 'x' {
		perms |= PermDelete
	}
	return perms, nil
}

// ValidPermission checks if a permission value is valid (0-7)
func ValidPermission(perms int) bool {
	return perms >= 0 && perms <= 7
}

// Action constants for clarity
const (
	ActionRead   = PermRead
	ActionWrite  = PermWrite
	ActionDelete = PermDelete
)
