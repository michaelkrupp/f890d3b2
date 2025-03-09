package logging

import (
	"io"
	"log/slog"
)

// NewNopLogger creates a logger that discards all output.
// Useful for testing or when logging needs to be disabled.
func NewNopLogger() Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
