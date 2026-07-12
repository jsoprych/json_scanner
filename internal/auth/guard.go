package auth

import (
	"database/sql"
	"fmt"
	"time"
)

// LoginGuard tracks per-account failed login attempts and lockout.
// Stores state in SQLite (or any *sql.DB compatible database).
type LoginGuard struct {
	db *sql.DB
}

// NewLoginGuard creates a login guard. Call once at startup.
func NewLoginGuard(db *sql.DB) (*LoginGuard, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS login_audit (
		user_id TEXT PRIMARY KEY,
		failures INTEGER NOT NULL DEFAULT 0,
		locked_until INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		return nil, err
	}
	return &LoginGuard{db: db}, nil
}

// Check returns (error message, allowed). If the account is locked,
// returns the reason and false. Otherwise returns true.
func (g *LoginGuard) Check(userID string) (string, bool) {
	var failures int
	var lockedUntil int64
	err := g.db.QueryRow("SELECT failures, locked_until FROM login_audit WHERE user_id = ?", userID).
		Scan(&failures, &lockedUntil)
	if err == sql.ErrNoRows {
		return "", true
	}

	now := time.Now().Unix()
	if lockedUntil > now {
		remaining := time.Duration(lockedUntil-now) * time.Second
		return fmt.Sprintf("Account locked. Try again in %s.", remaining.Round(time.Second)), false
	}
	return "", true
}

// RecordFailure increments the failure counter. Locks account after 5 failures.
func (g *LoginGuard) RecordFailure(userID string) {
	var failures int
	g.db.QueryRow("SELECT failures FROM login_audit WHERE user_id = ?", userID).Scan(&failures)
	failures++
	locked := int64(0)
	if failures >= 5 {
		locked = time.Now().Unix() + 900 // 15 minutes
	}
	g.db.Exec("INSERT OR REPLACE INTO login_audit (user_id, failures, locked_until) VALUES (?, ?, ?)",
		userID, failures, locked)
}

// RecordSuccess clears the failure counter.
func (g *LoginGuard) RecordSuccess(userID string) {
	g.db.Exec("INSERT OR REPLACE INTO login_audit (user_id, failures, locked_until) VALUES (?, 0, 0)", userID)
}
