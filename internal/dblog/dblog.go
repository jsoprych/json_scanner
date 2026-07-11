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
type DB struct {
	db  *sql.DB
	log *slog.Logger
}

// New creates a loggable DB wrapper.
func New(db *sql.DB, log *slog.Logger) *DB {
	return &DB{db: db, log: log.With("component", "db")}
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
		d.log.Error("db exec failed", "query", query, "args", args, "duration", dur, "error", err)
	} else {
		d.log.Debug("db exec", "query", query, "duration", dur)
	}
	return res, err
}

// ExecContext executes a write query with context and logging.
func (d *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	res, err := d.db.ExecContext(ctx, query, args...)
	dur := time.Since(start)
	if err != nil {
		d.log.Error("db exec failed", "query", query, "args", args, "duration", dur, "error", err)
	} else {
		d.log.Debug("db exec", "query", query, "duration", dur)
	}
	return res, err
}

// Query executes a read query with logging.
func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := d.db.Query(query, args...)
	dur := time.Since(start)
	if err != nil {
		d.log.Error("db query failed", "query", query, "args", args, "duration", dur, "error", err)
	} else {
		d.log.Debug("db query", "query", query, "duration", dur)
	}
	return rows, err
}

// QueryContext executes a read query with context and logging.
func (d *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := d.db.QueryContext(ctx, query, args...)
	dur := time.Since(start)
	if err != nil {
		d.log.Error("db query failed", "query", query, "args", args, "duration", dur, "error", err)
	} else {
		d.log.Debug("db query", "query", query, "duration", dur)
	}
	return rows, err
}

// QueryRow executes a single-row query with logging.
func (d *DB) QueryRow(query string, args ...any) *sql.Row {
	start := time.Now()
	row := d.db.QueryRow(query, args...)
	d.log.Debug("db query row", "query", query, "duration", time.Since(start))
	return row
}

// QueryRowContext executes a single-row query with context and logging.
func (d *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	start := time.Now()
	row := d.db.QueryRowContext(ctx, query, args...)
	d.log.Debug("db query row", "query", query, "duration", time.Since(start))
	return row
}

// Begin starts a transaction with logging.
func (d *DB) Begin() (*sql.Tx, error) {
	start := time.Now()
	tx, err := d.db.Begin()
	dur := time.Since(start)
	if err != nil {
		d.log.Error("db begin failed", "duration", dur, "error", err)
	} else {
		d.log.Debug("db begin", "duration", dur)
	}
	return tx, err
}

// Prepare prepares a statement with logging.
func (d *DB) Prepare(query string) (*sql.Stmt, error) {
	start := time.Now()
	stmt, err := d.db.Prepare(query)
	dur := time.Since(start)
	if err != nil {
		d.log.Error("db prepare failed", "query", query, "duration", dur, "error", err)
	} else {
		d.log.Debug("db prepare", "query", query, "duration", dur)
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
