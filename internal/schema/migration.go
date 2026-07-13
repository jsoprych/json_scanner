package schema

import (
	"database/sql"
	"fmt"
)

// Migration scripts for scanner.db schema evolution

const (
	// Version 1: Initial schema (snapshot table)
	Version1 = 1
	// Version 2: Groups, permissions, saved results
	Version2 = 2
	// Version 3: Roles and throttling
	Version3 = 3
	// Version 4: pass_hash column on users
	Version4 = 4
	// Version 5: new indicators (PSAR, Aroon, Keltner, CMF, Ultimate Oscillator)
	Version5 = 5
	// Version 6: NLP per-user config
	Version6 = 6
)

// CurrentVersion is the latest schema version
const CurrentVersion = Version6

// Migrate runs all pending migrations
func Migrate(db *sql.DB) error {
	// Create schema_version table if not exists
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_version table: %w", err)
	}

	// Get current version
	var currentVersion int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("get current version: %w", err)
	}

	// Run pending migrations
	if currentVersion < Version2 {
		if err := migrateToV2(db); err != nil {
			return fmt.Errorf("migrate to v2: %w", err)
		}
	}

	if currentVersion < Version3 {
		if err := migrateToV3(db); err != nil {
			return fmt.Errorf("migrate to v3: %w", err)
		}
	}

	if currentVersion < Version4 {
		if err := migrateToV4(db); err != nil {
			return fmt.Errorf("migrate to v4: %w", err)
		}
	}

	if currentVersion < Version5 {
		if err := migrateToV5(db); err != nil {
			return fmt.Errorf("migrate to v5: %w", err)
		}
	} else {
		if err := migrateToV5(db); err != nil {
			return fmt.Errorf("repair v5 columns: %w", err)
		}
	}

	if currentVersion < Version6 {
		if err := migrateToV6(db); err != nil {
			return fmt.Errorf("migrate to v6: %w", err)
		}
	} else {
		if err := migrateToV6(db); err != nil {
			return fmt.Errorf("repair v6 columns: %w", err)
		}
	}

	return nil
}

// migrateToV2 adds groups, permissions, and saved results
func migrateToV2(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Groups table
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			owner_id TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("create groups table: %w", err)
	}

	// Group membership table
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS group_members (
			group_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			joined_at INTEGER NOT NULL,
			PRIMARY KEY (group_id, user_id),
			FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("create group_members table: %w", err)
	}

	// Create index for user membership lookups
	_, err = tx.Exec(`
		CREATE INDEX IF NOT EXISTS idx_group_members_user 
		ON group_members(user_id)
	`)
	if err != nil {
		return fmt.Errorf("create group_members index: %w", err)
	}

	// Saved results table with Linux-style permissions
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS saved_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			study_id TEXT NOT NULL,
			snapshot_date INTEGER NOT NULL,
			results_json TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			name TEXT,
			notes TEXT,
			perm_owner INTEGER NOT NULL DEFAULT 7,
			perm_group INTEGER NOT NULL DEFAULT 0,
			perm_all INTEGER NOT NULL DEFAULT 0,
			group_id TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("create saved_results table: %w", err)
	}

	// Index for user results lookups
	_, err = tx.Exec(`
		CREATE INDEX IF NOT EXISTS idx_saved_results_user 
		ON saved_results(user_id, created_at DESC)
	`)
	if err != nil {
		return fmt.Errorf("create saved_results user index: %w", err)
	}

	// Index for date-based queries
	_, err = tx.Exec(`
		CREATE INDEX IF NOT EXISTS idx_saved_results_date 
		ON saved_results(snapshot_date DESC)
	`)
	if err != nil {
		return fmt.Errorf("create saved_results date index: %w", err)
	}

	// ACLs table for explicit per-user sharing
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS result_acls (
			result_id INTEGER NOT NULL,
			user_id TEXT NOT NULL,
			permission INTEGER NOT NULL,
			granted_at INTEGER NOT NULL,
			granted_by TEXT NOT NULL,
			PRIMARY KEY (result_id, user_id),
			FOREIGN KEY (result_id) REFERENCES saved_results(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("create result_acls table: %w", err)
	}

	// Record migration
	_, err = tx.Exec(`
		INSERT INTO schema_version (version, applied_at) 
		VALUES (?, strftime('%s', 'now'))
	`, Version2)
	if err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}

// migrateToV3 adds roles, users table, and throttling tables
func migrateToV3(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Users table (SQLite-first — source of truth for throttle/permissions queries)
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			role_id TEXT NOT NULL DEFAULT 'user',
			pass_hash TEXT NOT NULL DEFAULT '',
			disabled INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return fmt.Errorf("create users table: %w", err)
	}

	// Studies table (SQLite-first — replaces studies.jsonl)
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS studies (
			key TEXT PRIMARY KEY,
			owner TEXT NOT NULL DEFAULT 'global',
			visibility TEXT NOT NULL DEFAULT 'private',
			group_name TEXT NOT NULL DEFAULT '',
			tier TEXT NOT NULL DEFAULT 'free',
			title TEXT NOT NULL DEFAULT '',
			emoji TEXT NOT NULL DEFAULT '',
			where_clause TEXT NOT NULL DEFAULT '',
			order_by TEXT NOT NULL DEFAULT '',
			limit_num INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return fmt.Errorf("create studies table: %w", err)
	}

	// Subscriptions table (SQLite-first — replaces subscriptions.jsonl)
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS subscriptions (
			user_id TEXT NOT NULL,
			study_key TEXT NOT NULL,
			PRIMARY KEY (user_id, study_key)
		)
	`)
	if err != nil {
		return fmt.Errorf("create subscriptions table: %w", err)
	}

	// Roles table
	_, err = tx.Exec(`
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
	if err != nil {
		return fmt.Errorf("create roles table: %w", err)
	}

	// Add role_id to users table
	_, err = tx.Exec(`
		ALTER TABLE users ADD COLUMN role_id TEXT REFERENCES roles(id)
	`)
	if err != nil {
		if !isColumnExistsError(err) {
			return fmt.Errorf("add role_id to users: %w", err)
		}
	}

	// User-specific limit overrides
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS user_limits (
			user_id TEXT PRIMARY KEY,
			limits_json TEXT NOT NULL,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("create user_limits table: %w", err)
	}

	// Usage tracking
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS usage_tracking (
			user_id TEXT NOT NULL,
			date TEXT NOT NULL,
			api_calls INTEGER DEFAULT 0,
			studies_created INTEGER DEFAULT 0,
			results_saved INTEGER DEFAULT 0,
			replays_run INTEGER DEFAULT 0,
			PRIMARY KEY (user_id, date)
		)
	`)
	if err != nil {
		return fmt.Errorf("create usage_tracking table: %w", err)
	}

	// Rate limit tracking
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS rate_limits (
			user_id TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			action TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("create rate_limits table: %w", err)
	}

	// Index for rate limit cleanup
	_, err = tx.Exec(`
		CREATE INDEX IF NOT EXISTS idx_rate_limits_timestamp
		ON rate_limits(timestamp)
	`)
	if err != nil {
		return fmt.Errorf("create rate_limits index: %w", err)
	}

	// Record migration
	_, err = tx.Exec(`
		INSERT INTO schema_version (version, applied_at) 
		VALUES (?, strftime('%s', 'now'))
	`, Version3)
	if err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}

// migrateToV4 adds pass_hash column to users for password persistence.
func migrateToV4(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`ALTER TABLE users ADD COLUMN pass_hash TEXT NOT NULL DEFAULT ''`)
	if err != nil {
		if !isColumnExistsError(err) {
			return fmt.Errorf("add pass_hash to users: %w", err)
		}
	}

	_, err = tx.Exec(`
		INSERT INTO schema_version (version, applied_at) 
		VALUES (?, strftime('%s', 'now'))
	`, Version4)
	if err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}

// isColumnExistsError checks if error is "column already exists"
func isColumnExistsError(err error) bool {
	return err != nil && (contains(err.Error(), "duplicate column") || 
		contains(err.Error(), "already exists"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		findSubstring(s, substr)))
}

func tableExists(tx *sql.Tx, name string) bool {
	var count int
	err := tx.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&count)
	return err == nil && count > 0
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetVersion returns the current schema version
func GetVersion(db *sql.DB) (int, error) {
	var version int
	err := db.QueryRow(`
		SELECT COALESCE(MAX(version), 0) 
		FROM schema_version
	`).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

// migrateToV5 adds new indicator columns to the snapshot table.
func migrateToV5(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil { return err }
	defer tx.Rollback()

	// If the table doesn't exist yet, ensureTable() in snapshot package will
	// create it with the full schema — record version and move on.
	if !tableExists(tx, "snapshot") {
		_, err = tx.Exec("INSERT OR IGNORE INTO schema_version (version, applied_at) VALUES (?, strftime('%s', 'now'))", Version5)
		if err != nil { return fmt.Errorf("record migration: %w", err) }
		return tx.Commit()
	}

	for _, col := range []string{
		"psar REAL", "aroon_up REAL", "aroon_down REAL", "aroon_osc REAL",
		"keltner_upper REAL", "keltner_mid REAL", "keltner_lower REAL",
		"cmf20 REAL", "ultimate_osc REAL",
	} {
		_, err = tx.Exec("ALTER TABLE snapshot ADD COLUMN " + col)
		if err != nil {
			if !isColumnExistsError(err) {
				return fmt.Errorf("add column %s: %w", col, err)
			}
		}
	}
	_, err = tx.Exec("INSERT OR IGNORE INTO schema_version (version, applied_at) VALUES (?, strftime('%s', 'now'))", Version5)
	if err != nil { return fmt.Errorf("record migration: %w", err) }
	return tx.Commit()
}

// migrateToV6 adds NLP per-user configuration columns.
func migrateToV6(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil { return err }
	defer tx.Rollback()

	for _, col := range []string{
		"nlp_enabled INTEGER NOT NULL DEFAULT 1",
		"nlp_daily_limit INTEGER NOT NULL DEFAULT 10",
	} {
		_, err = tx.Exec("ALTER TABLE users ADD COLUMN " + col)
		if err != nil {
			if !isColumnExistsError(err) {
				return fmt.Errorf("add column %s: %w", col, err)
			}
		}
	}
	_, err = tx.Exec("INSERT OR IGNORE INTO schema_version (version, applied_at) VALUES (?, strftime('%s', 'now'))", Version6)
	if err != nil { return fmt.Errorf("record migration: %w", err) }
	return tx.Commit()
}
