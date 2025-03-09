package context

import (
	"context"
)

const contextKeyTraceID = contextKey("traceID")

// TraceIDFromContext extracts the trace ID from the context.
// Returns the trace ID and true if present, or empty string and false if not present.
func TraceIDFromContext(ctx context.Context) (string, bool) {
	traceID, ok := ctx.Value(contextKeyTraceID).(string)

	return traceID, ok
}

// WithTraceID creates a new context with the given trace ID value.
// This context can be used to track a request through different systems.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, contextKeyTraceID, traceID)
}
