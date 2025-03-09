package http

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/mkrupp/homecase-michael/internal/infra/logging"
)

// RescueingMiddleware creates middleware that recovers from panics in HTTP handlers.
// It logs the panic and stack trace, then returns a 500 Internal Server Error to the client.
func RescueingMiddleware(next http.Handler, log logging.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func(ctx context.Context) {
			if p := recover(); p != nil {
				log.ErrorContext(ctx, "request panic", slog.Group("http",
					"uri", r.RequestURI,
					"method", r.Method,
				), slog.Group("error",
					"panic", p,
					"stack", string(debug.Stack()),
				))
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}(r.Context())
		next.ServeHTTP(w, r)
	})
}
