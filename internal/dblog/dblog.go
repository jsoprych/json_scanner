// Package dblog provides a centralized, loggable database wrapper for all stores.
// Every query — read or write — is logged with duration and result.
package dblog

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

// DB wraps *sql.DB with structured logging.
// Log level: "all" (default), "slow" (>5ms), "error" (only failures), "off".
type DB struct {
	db    *sql.DB
	log   *slog.Logger
	level string
}

// New creates a loggable DB wrapper.
func New(db *sql.DB, log *slog.Logger) *DB {
	return &DB{db: db, log: log.With("component", "db"), level: "all"}
}

// SetLogLevel tunes verbosity at runtime: "all", "slow", "error", "off".
func (d *DB) SetLogLevel(level string) { d.level = level }

func (d *DB) logDebug(dur time.Duration, msg, query string, args ...any) {
	switch d.level {
	case "off":
		return
	case "error":
		return
	case "slow":
		if dur < 5*time.Millisecond {
			return
		}
	}
	d.log.Debug(msg, "query", query, "args", args, "duration", dur)
}

func (d *DB) logErr(dur time.Duration, query string, err error, args ...any) {
	if d.level == "off" {
		return
	}
	d.log.Error("db "+query+" failed", "query", query, "args", args, "duration", dur, "error", err)
}

// DB returns the underlying *sql.DB for use with existing stores.
func (d *DB) DB() *sql.DB { return d.db }

// Log returns the logger.
func (d *DB) Log() *slog.Logger { return d.log }

// Exec executes a write query with logging.
func (d *DB) Exec(query string, args ...any) (sql.Result, error) {
	start := time.Now()
	res, err := d.db.Exec(query, args...)
	dur := time.Since(start)
	if err != nil {
		d.logErr(dur, query, err, args...)
	} else {
		d.logDebug(dur, "db exec", query, args...)
	}
	return res, err
}

// ExecContext executes a write query with context and logging.
func (d *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	res, err := d.db.ExecContext(ctx, query, args...)
	dur := time.Since(start)
	if err != nil {
		d.logErr(dur, query, err, args...)
	} else {
		d.logDebug(dur, "db exec", query, args...)
	}
	return res, err
}

// Query executes a read query with logging.
func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := d.db.Query(query, args...)
	dur := time.Since(start)
	if err != nil {
		d.logErr(dur, query, err, args...)
	} else {
		d.logDebug(dur, "db query", query, args...)
	}
	return rows, err
}

// QueryContext executes a read query with context and logging.
func (d *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := d.db.QueryContext(ctx, query, args...)
	dur := time.Since(start)
	if err != nil {
		d.logErr(dur, query, err, args...)
	} else {
		d.logDebug(dur, "db query", query, args...)
	}
	return rows, err
}

// QueryRow executes a single-row query with logging.
func (d *DB) QueryRow(query string, args ...any) *sql.Row {
	start := time.Now()
	row := d.db.QueryRow(query, args...)
	d.logDebug(time.Since(start), "db query row", query, args...)
	return row
}

// QueryRowContext executes a single-row query with context and logging.
func (d *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	start := time.Now()
	row := d.db.QueryRowContext(ctx, query, args...)
	d.logDebug(time.Since(start), "db query row", query, args...)
	return row
}

// Begin starts a transaction with logging.
func (d *DB) Begin() (*sql.Tx, error) {
	start := time.Now()
	tx, err := d.db.Begin()
	dur := time.Since(start)
	if err != nil {
		d.logErr(dur, "BEGIN", err)
	} else {
		d.logDebug(dur, "db begin", "BEGIN")
	}
	return tx, err
}

// Prepare prepares a statement with logging.
func (d *DB) Prepare(query string) (*sql.Stmt, error) {
	start := time.Now()
	stmt, err := d.db.Prepare(query)
	dur := time.Since(start)
	if err != nil {
		d.logErr(dur, query, err)
	} else {
		d.logDebug(dur, "db prepare", query)
	}
	return stmt, err
}

// Close closes the database with logging.
func (d *DB) Close() error {
	d.log.Debug("db close")
	return d.db.Close()
}

// Stats logs database statistics.
func (d *DB) Stats() {
	stats := d.db.Stats()
	d.log.Info("db stats",
		"max_open_conns", stats.MaxOpenConnections,
		"open_conns", stats.OpenConnections,
		"in_use", stats.InUse,
		"idle", stats.Idle,
		"wait_count", stats.WaitCount,
		"wait_duration", stats.WaitDuration,
	)
}
