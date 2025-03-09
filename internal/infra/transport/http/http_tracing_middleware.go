package http

import (
	"net/http"

	context_ "github.com/mkrupp/homecase-michael/internal/infra/context"
	"github.com/mkrupp/homecase-michael/internal/util/encoding"
	"github.com/mkrupp/homecase-michael/internal/util/uuid"
)

const TraceIDHeader = "X-Request-ID"

// TracingMiddleware creates middleware that adds request tracing.
// It uses the X-Request-ID header if present, otherwise generates a new UUIDv7.
// The trace ID is added to the request context.
func TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context_.WithTraceID(r.Context(), getTraceID(r))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getTraceID(r *http.Request) string {
	if traceID := r.Header.Get(TraceIDHeader); traceID != "" {
		return traceID
	}

	uuid, err := uuid.New(uuid.UUIDv7)
	if err != nil {
		return ""
	}

	return encoding.EncodeCrockfordB32LC(uuid.Bytes())
}
