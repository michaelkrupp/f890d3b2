package context

import (
	"context"
)

const contextKeyUsername = contextKey("username")

// UsernameFromContext extracts the username from the context.
// Returns the username and true if present, or empty string and false if not present.
func UsernameFromContext(ctx context.Context) (string, bool) {
	traceID, ok := ctx.Value(contextKeyUsername).(string)

	return traceID, ok
}

// WithUsername creates a new context with the given username value.
// This context can be used to track the authenticated user throughout a request.
func WithUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, contextKeyUsername, username)
}
