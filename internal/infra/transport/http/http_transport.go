package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/mkrupp/homecase-michael/internal/infra/logging"
)

// HTTPTransportConfig contains configuration parameters for HTTP servers.
type HTTPTransportConfig struct {
	// ServerAddr is the network address to listen on
	ServerAddr string `env:"SERVER_ADDR" default:":8080"`
	// ReadHeaderTimeout is the timeout in seconds for reading request headers
	ReadHeaderTimeout int64 `env:"READ_HEADER_TIMEOUT" default:"5"`

	ReadTimeout  int64 `env:"READ_TIMEOUT" default:"5"`
	WriteTimeout int64 `env:"WRITE_TIMEOUT" default:"5"`
}

// HTTPTransport defines the interface for HTTP handlers that can serve requests.
type HTTPTransport interface {
	http.Handler
}

// HTTPHandlerFunc converts an HTTPTransport into a standard http.HandlerFunc.
// This allows using HTTPTransport implementations with standard HTTP middleware.
func HTTPHandlerFunc(handler HTTPTransport) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}
}

// ListenAndServe starts an HTTP server with the given handler and configuration.
// It sets up standard middleware for logging, tracing, and panic recovery.
// Returns an error if the server fails to start or encounters an error while running.
func ListenAndServe(ctx context.Context, handler HTTPTransport, cfg HTTPTransportConfig) (err error) {
	log := logging.GetLogger("infra.transport.http")

	handler = RescueingMiddleware(handler, log)
	handler = LoggingMiddleware(handler, log)
	handler = TracingMiddleware(handler)

	//nolint:exhaustruct
	server := &http.Server{
		Addr:              cfg.ServerAddr,
		Handler:           handler,
		ErrorLog:          logging.GetLogLogger(log, logging.LevelError),
		ReadHeaderTimeout: time.Duration(cfg.ReadHeaderTimeout * int64(time.Second)),
		ReadTimeout:       time.Duration(cfg.ReadTimeout * int64(time.Second)),
		WriteTimeout:      time.Duration(cfg.WriteTimeout * int64(time.Second)),
	}
	defer server.Close()

	sock, err := net.Listen("tcp", cfg.ServerAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer sock.Close()

	log.DebugContext(ctx, "listening", "addr", cfg.ServerAddr)

	if err := server.Serve(sock); err != nil {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}
