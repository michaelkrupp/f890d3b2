package logging

import (
	"context"
	"log/slog"

	context_ "github.com/mkrupp/homecase-michael/internal/infra/context"
)

// TracingHandler wraps another slog.Handler to add trace IDs from the context
// to all log records.
type TracingHandler struct {
	h slog.Handler
}

var _ slog.Handler = (*TracingHandler)(nil)

// NewTracingHandler creates a new TracingHandler wrapping the given handler.
func NewTracingHandler(h slog.Handler) *TracingHandler {
	return &TracingHandler{h: h}
}

// Handle implements slog.Handler by adding trace ID information if available
// in the context before delegating to the wrapped handler.
func (h *TracingHandler) Handle(ctx context.Context, r slog.Record) error {
	if requestID, ok := context_.TraceIDFromContext(ctx); ok {
		r.AddAttrs(slog.Group("trace",
			slog.String("id", requestID),
		))
	}

	//nolint:wrapcheck
	return h.h.Handle(ctx, r)
}

// WithAttrs implements slog.Handler.WithAttrs.
func (h *TracingHandler) WithAttrs(attrs []slog.Attr) Handler {
	return NewTracingHandler(h.h.WithAttrs(attrs))
}

// WithGroup implements slog.Handler.WithGroup.
func (h *TracingHandler) WithGroup(name string) Handler {
	return NewTracingHandler(h.h.WithGroup(name))
}

// Enabled implements slog.Handler.Enabled.
func (h *TracingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.h.Enabled(ctx, level)
}
