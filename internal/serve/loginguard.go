package serve

import (
	"database/sql"
	"fmt"
	"time"
)

// loginGuard manages per-account login attempt tracking and lockout.
type loginGuard struct {
	db *sql.DB
}

func newLoginGuard(db *sql.DB) (*loginGuard, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS login_audit (
		user_id TEXT PRIMARY KEY,
		failures INTEGER NOT NULL DEFAULT 0,
		locked_until INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		return nil, err
	}
	return &loginGuard{db: db}, nil
}

// Check returns an error message if the account is locked, or nil if login can proceed.
func (g *loginGuard) Check(userID string) (string, bool) {
	var failures int
	var lockedUntil int64
	err := g.db.QueryRow("SELECT failures, locked_until FROM login_audit WHERE user_id = ?", userID).
		Scan(&failures, &lockedUntil)
	if err == sql.ErrNoRows {
		return "", true // no record, allow
	}

	now := time.Now().Unix()
	if lockedUntil > now {
		remaining := time.Duration(lockedUntil-now) * time.Second
		return fmt.Sprintf("Account locked. Try again in %s.", remaining.Round(time.Second)), false
	}
	return "", true
}

// RecordFailure increments the failure counter and locks if threshold reached.
func (g *loginGuard) RecordFailure(userID string) {
	var failures int
	var lockedUntil int64
	err := g.db.QueryRow("SELECT failures, locked_until FROM login_audit WHERE user_id = ?", userID).
		Scan(&failures, &lockedUntil)
	if err == sql.ErrNoRows {
		failures = 0
	}
	failures++
	now := time.Now().Unix()
	locked := int64(0)
	if failures >= 5 {
		locked = now + 900 // 15 minutes
	}
	g.db.Exec("INSERT OR REPLACE INTO login_audit (user_id, failures, locked_until) VALUES (?, ?, ?)",
		userID, failures, locked)
}

// RecordSuccess clears the failure counter for the user.
func (g *loginGuard) RecordSuccess(userID string) {
	g.db.Exec("INSERT OR REPLACE INTO login_audit (user_id, failures, locked_until) VALUES (?, 0, 0)", userID)
}
