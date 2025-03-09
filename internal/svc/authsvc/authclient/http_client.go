package authclient

import (
	"context"
	"fmt"
	"io"
	"net/http"

	context_ "github.com/mkrupp/homecase-michael/internal/infra/context"
	"github.com/mkrupp/homecase-michael/internal/infra/logging"
)

const (
	TraceIDHeader       = "X-Request-ID"
	AuthorizationHeader = "Authorization"
)

// HTTPClientConfig holds configuration for the HTTP auth client.
type HTTPClientConfig struct {
	// AuthURL is the endpoint for token validation requests
	AuthURL string `env:"AUTH_URL" default:"http://localhost:8080/auth/validate"`
}

// HTTPClient implements AuthClient using HTTP requests to validate tokens.
type HTTPClient struct {
	httpClient *http.Client
	log        logging.Logger
	cfg        HTTPClientConfig
}

var _ AuthClient = (*HTTPClient)(nil)

// NewHTTPClient creates a new HTTPClient with the given configuration.
// If httpClient is nil, http.DefaultClient will be used.
func NewHTTPClient(
	cfg HTTPClientConfig,
	httpClient *http.Client,
) *HTTPClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &HTTPClient{
		httpClient: httpClient,
		log:        logging.GetLogger("svc.authsvc.http_client"),
		cfg:        cfg,
	}
}

// Validate implements AuthClient.Validate by making an HTTP request to the configured
// auth service endpoint. The token is sent in the Authorization header.
func (ht *HTTPClient) Validate(ctx context.Context, token string) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ht.cfg.AuthURL, nil)
	if err != nil {
		return "", false, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set(AuthorizationHeader, token)

	if traceID, ok := context_.TraceIDFromContext(ctx); ok {
		req.Header.Set(TraceIDHeader, traceID)
	}

	resp, err := ht.httpClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, nil
	}

	username, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("read string: %w", err)
	}

	return string(username), true, nil
}
