// Package telemetry provides the scanner's structured JSON logger.
//
// Logs go to STDERR so that STDOUT carries only the signal stream (JSONL),
// keeping the two cleanly separable for piping.
package telemetry

import (
	"io"
	"log/slog"
)

// New returns a JSON logger writing to w at Info level.
func New(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
