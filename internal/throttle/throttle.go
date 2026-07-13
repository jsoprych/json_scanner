package throttle

import (
	"database/sql"
	"cetus-marketdata-scanner/internal/dblog"
	"encoding/json"
	"fmt"
	"time"

	"cetus-marketdata-scanner/internal/roles"
)

// Throttler enforces rate limits and quotas
type Throttler struct {
	db        *dblog.DB
	roleStore *roles.Store
}

// NewThrottler creates a new throttler
func NewThrottler(db *dblog.DB, roleStore *roles.Store) *Throttler {
	return &Throttler{
		db:        db,
		roleStore: roleStore,
	}
}

// DB returns the underlying database for tracking queries.
func (t *Throttler) DB() *dblog.DB { return t.db }

// Init initializes throttling tables
func (t *Throttler) Init() error {
	// User-specific limit overrides
	_, err := t.db.Exec(`
		CREATE TABLE IF NOT EXISTS user_limits (
			user_id TEXT PRIMARY KEY REFERENCES users(id),
			limits_json TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	// Usage tracking
	_, err = t.db.Exec(`
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
		return err
	}

	// Rate limit tracking (in-memory window)
	_, err = t.db.Exec(`
		CREATE TABLE IF NOT EXISTS rate_limits (
			user_id TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			action TEXT NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	// Index for cleanup
	_, err = t.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_rate_limits_timestamp
		ON rate_limits(timestamp)
	`)
	return err
}

// CheckCreate checks quota BEFORE creating a resource. Returns clear error or nil.
func (t *Throttler) CheckCreate(userID, resource string) error {
	limits, err := t.GetEffectiveLimits(userID)
	if err != nil {
		return fmt.Errorf("cannot check limits: %w", err)
	}
	var current, max int
	switch resource {
	case "study":
		err = t.db.QueryRow("SELECT COUNT(*) FROM studies WHERE owner = ?", userID).Scan(&current)
		if err != nil {
			current = 0
		}
		max = limits.MaxStudies
	case "result":
		err = t.db.QueryRow("SELECT COUNT(*) FROM saved_results WHERE user_id = ?", userID).Scan(&current)
		if err != nil {
			current = 0
		}
		max = limits.MaxSavedResults
	case "group":
		err = t.db.QueryRow("SELECT COUNT(*) FROM groups WHERE owner_id = ?", userID).Scan(&current)
		if err != nil {
			current = 0
		}
		max = limits.MaxGroups
	default:
		return nil
	}
	if current >= max {
		return fmt.Errorf("%s limit: %d of %d used", resource, current, max)
	}
	return nil
}

// TrackCreate tracks resource creation AFTER successful create.
func (t *Throttler) TrackCreate(userID, resource string) {
	_ = t.TrackResourceUsage(userID, resource)
}

// CheckRateLimit checks if user has exceeded rate limits
func (t *Throttler) CheckRateLimit(userID, action string) error {
	// Get user role from SQLite (SQLite-first)
	var roleID string
	err := t.db.QueryRow("SELECT role_id FROM users WHERE id = ?", userID).Scan(&roleID)
	if err != nil {
		return fmt.Errorf("get user role: %w", err)
	}

	role, err := t.roleStore.Get(roleID)
	if err != nil {
		return fmt.Errorf("get role: %w", err)
	}
	if role == nil {
		return fmt.Errorf("role not found: %s", roleID)
	}

	// Admin bypass
	if role.BypassThrottling {
		return nil
	}

	limits, err := t.GetEffectiveLimits(userID)
	if err != nil {
		return err
	}

	now := time.Now().Unix()

	// Check per-minute limit
	minuteAgo := now - 60
	var minuteCount int
	err = t.db.QueryRow(`
		SELECT COUNT(*) FROM rate_limits
		WHERE user_id = ? AND timestamp > ?
	`, userID, minuteAgo).Scan(&minuteCount)
	if err != nil {
		return fmt.Errorf("check minute limit: %w", err)
	}
	if minuteCount >= limits.APICallsPerMinute {
		return fmt.Errorf("rate limit exceeded: %d calls per minute", limits.APICallsPerMinute)
	}

	// Check per-hour limit
	hourAgo := now - 3600
	var hourCount int
	err = t.db.QueryRow(`
		SELECT COUNT(*) FROM rate_limits
		WHERE user_id = ? AND timestamp > ?
	`, userID, hourAgo).Scan(&hourCount)
	if err != nil {
		return fmt.Errorf("check hour limit: %w", err)
	}
	if hourCount >= limits.APICallsPerHour {
		return fmt.Errorf("rate limit exceeded: %d calls per hour", limits.APICallsPerHour)
	}

	// Check per-day limit
	today := time.Now().Format("2006-01-02")
	var dayCount int
	err = t.db.QueryRow(`
		SELECT api_calls FROM usage_tracking
		WHERE user_id = ? AND date = ?
	`, userID, today).Scan(&dayCount)
	if err == sql.ErrNoRows {
		dayCount = 0
	} else if err != nil {
		return fmt.Errorf("check day limit: %w", err)
	}
	if dayCount >= limits.APICallsPerDay {
		return fmt.Errorf("daily quota exceeded: %d calls per day", limits.APICallsPerDay)
	}

	return nil
}

// TrackAPIUsage records an API call
func (t *Throttler) TrackAPIUsage(userID string) error {
	now := time.Now()
	timestamp := now.Unix()
	today := now.Format("2006-01-02")

	// Record in rate_limits
	_, err := t.db.Exec(`
		INSERT INTO rate_limits (user_id, timestamp, action)
		VALUES (?, ?, 'api_call')
	`, userID, timestamp)
	if err != nil {
		return fmt.Errorf("record rate limit: %w", err)
	}

	// Update daily usage
	_, err = t.db.Exec(`
		INSERT INTO usage_tracking (user_id, date, api_calls)
		VALUES (?, ?, 1)
		ON CONFLICT(user_id, date) DO UPDATE SET api_calls = api_calls + 1
	`, userID, today)
	if err != nil {
		return fmt.Errorf("update daily usage: %w", err)
	}

	// Cleanup old rate limits (older than 1 hour)
	oneHourAgo := timestamp - 3600
	t.db.Exec("DELETE FROM rate_limits WHERE timestamp < ?", oneHourAgo)

	return nil
}

// TrackResourceUsage records resource creation (study, result, etc.)
func (t *Throttler) TrackResourceUsage(userID, resource string) error {
	today := time.Now().Format("2006-01-02")

	var column string
	switch resource {
	case "study":
		column = "studies_created"
	case "result":
		column = "results_saved"
	case "replay":
		column = "replays_run"
	default:
		return fmt.Errorf("unknown resource: %s", resource)
	}

	query := fmt.Sprintf(`
		INSERT INTO usage_tracking (user_id, date, %s)
		VALUES (?, ?, 1)
		ON CONFLICT(user_id, date) DO UPDATE SET %s = %s + 1
	`, column, column, column)

	_, err := t.db.Exec(query, userID, today)
	return err
}

// GetEffectiveLimits returns limits (role defaults + user overrides)
func (t *Throttler) GetEffectiveLimits(userID string) (*roles.Limits, error) {
	// Check for user-specific overrides
	var limitsJSON string
	err := t.db.QueryRow("SELECT limits_json FROM user_limits WHERE user_id = ?", userID).Scan(&limitsJSON)
	if err == nil {
		var limits roles.Limits
		if err := parseJSON(limitsJSON, &limits); err == nil {
			return &limits, nil
		}
	}

	// Fall back to role defaults from SQLite users table
	var roleID string
	err = t.db.QueryRow("SELECT role_id FROM users WHERE id = ?", userID).Scan(&roleID)
	if err != nil {
		return nil, fmt.Errorf("get user role: %w", err)
	}

	role, err := t.roleStore.Get(roleID)
	if err != nil {
		return nil, fmt.Errorf("get role: %w", err)
	}
	if role == nil {
		return nil, fmt.Errorf("role not found: %s", roleID)
	}

	return &role.Limits, nil
}

// SetUserLimits sets user-specific limit overrides
func (t *Throttler) SetUserLimits(userID string, limits roles.Limits) error {
	limitsJSON, err := toJSON(limits)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	_, err = t.db.Exec(`
		INSERT INTO user_limits (user_id, limits_json, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET limits_json = ?, updated_at = ?
	`, userID, limitsJSON, now, limitsJSON, now)

	return err
}

// GetUserUsage returns current usage for a user
func (t *Throttler) GetUserUsage(userID string) (map[string]int, error) {
	today := time.Now().Format("2006-01-02")

	var apiCalls, studies, results, replays int
	err := t.db.QueryRow(`
		SELECT api_calls, studies_created, results_saved, replays_run
		FROM usage_tracking
		WHERE user_id = ? AND date = ?
	`, userID, today).Scan(&apiCalls, &studies, &results, &replays)
	if err == sql.ErrNoRows {
		return map[string]int{
			"api_calls":     0,
			"studies":       0,
			"results":       0,
			"replays":       0,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	return map[string]int{
		"api_calls": apiCalls,
		"studies":   studies,
		"results":   results,
		"replays":   replays,
	}, nil
}

// Helper functions
func parseJSON(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}

func toJSON(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	return string(data), err
}
