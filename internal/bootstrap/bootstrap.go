package bootstrap

import (
	"fmt"
	"log/slog"

	"cetus-marketdata-scanner/internal/dblog"
	"cetus-marketdata-scanner/internal/roles"
	"cetus-marketdata-scanner/internal/user"
)

// Bootstrap creates default admin user if no users exist
func Bootstrap(db *dblog.DB, users *user.Store, rolesStore *roles.Store, log *slog.Logger) error {
	// Check if any users exist
	allUsers := users.All()
	if len(allUsers) > 0 {
		log.Info("users already exist, skipping bootstrap", "count", len(allUsers))
		return nil
	}

	log.Info("no users found, creating default admin user")

	// Get admin role
	adminRole, err := rolesStore.Get("admin")
	if err != nil {
		return fmt.Errorf("get admin role: %w", err)
	}
	if adminRole == nil {
		return fmt.Errorf("admin role not found")
	}

	// Create default admin user
	admin := user.User{
		ID:       "admin",
		Name:     "Administrator",
		RoleID:   "admin",
		Disabled: false,
	}
	admin.SetPassword("admin") // Default password - should be changed immediately

	if err := users.Create(admin); err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}

	log.Warn("default admin user created with password 'admin' - CHANGE THIS IMMEDIATELY")
	log.Info("admin user created", "id", admin.ID, "role", admin.RoleID)

	return nil
}
