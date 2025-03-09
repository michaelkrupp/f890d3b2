package http

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/mkrupp/homecase-michael/internal/infra/logging"
)

// LoggingMiddlewareResponseWriter wraps http.ResponseWriter to capture response metrics.
type LoggingMiddlewareResponseWriter struct {
	http.ResponseWriter
	StatusCode int
	BytesSent  int
}

func (w *LoggingMiddlewareResponseWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
	w.StatusCode = code
}

func (w *LoggingMiddlewareResponseWriter) Write(b []byte) (int, error) {
	w.BytesSent += len(b)

	n, err := w.ResponseWriter.Write(b)
	if err != nil {
		return n, fmt.Errorf("write: %w", err)
	}

	return n, nil
}

// LoggingMiddleware creates middleware that logs HTTP request and response details.
// It logs requests at DEBUG level and responses at a level determined by the status code:
// - 5xx: ERROR
// - 4xx: WARN
// - Other: INFO.
func LoggingMiddleware(next http.Handler, log logging.Logger) http.Handler {
	//nolint:varnamelen
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.DebugContext(r.Context(), "request", slog.Group("http",
			"uri", r.RequestURI,
			"method", r.Method,
		))

		mw := &LoggingMiddlewareResponseWriter{
			ResponseWriter: w,
			StatusCode:     http.StatusOK, // This is default if no response code is written
			BytesSent:      0,
		}

		next.ServeHTTP(mw, r)

		var level logging.Level

		switch {
		case mw.StatusCode >= http.StatusInternalServerError:
			level = logging.LevelError
		case mw.StatusCode >= http.StatusBadRequest:
			level = logging.LevelWarn
		default:
			level = logging.LevelInfo
		}

		log.Log(r.Context(), level, "response", slog.Group("http",
			"uri", r.RequestURI,
			"method", r.Method,
			"status", mw.StatusCode,
			"bytes_sent", mw.BytesSent,
		))
	})
}
