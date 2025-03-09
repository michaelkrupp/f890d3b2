package authsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mkrupp/homecase-michael/internal/domain"
	"github.com/mkrupp/homecase-michael/internal/infra/logging"
	http_ "github.com/mkrupp/homecase-michael/internal/infra/transport/http"
)

var (
	// ErrNoUsername is returned when the username is missing from the request.
	ErrNoUsername = errors.New("no username")
	// ErrNoPassword is returned when the password is missing from the request.
	ErrNoPassword = errors.New("no password")
)

// HTTPTransportConfig contains configuration parameters for the HTTP transport layer.
type HTTPTransportConfig struct {
	http_.HTTPTransportConfig
}

// HTTPTransport handles HTTP requests for the authentication service.
// It provides endpoints for user registration, login, and token validation.
type HTTPTransport struct {
	authSvc *AuthService
	log     logging.Logger
	cfg     HTTPTransportConfig
}

// NewHTTPTransport creates a new HTTPTransport instance with the given configuration.
// It requires an AuthService for handling authentication operations.
func NewHTTPTransport(
	authSvc *AuthService,
	cfg HTTPTransportConfig,
) *HTTPTransport {
	return &HTTPTransport{
		authSvc: authSvc,
		log:     logging.GetLogger("svc.imagesvc.http_transport"),
		cfg:     cfg,
	}
}

// ServeHTTP implements http.Handler and sets up routes for the auth service endpoints:
// - POST /auth/register: Register a new user
// - POST /auth/login: Login and get an auth token
// - POST /auth/validate: Validate an auth token.
func (ht *HTTPTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/register", ht.HandleRegister)
	mux.HandleFunc("POST /auth/login", ht.HandleLogin)
	mux.HandleFunc("POST /auth/validate", ht.HandleValidate)
	mux.ServeHTTP(w, r)
}

var _ http_.HTTPTransport = (*HTTPTransport)(nil)

// HandleRegister processes user registration requests.
// Expects form parameters: username, password.
func (ht *HTTPTransport) HandleRegister(w http.ResponseWriter, r *http.Request) {
	_ = ht.handleRegister(w, r)
}

func (ht *HTTPTransport) handleRegister(w http.ResponseWriter, r *http.Request) (err error) {
	log := ht.log.With(logging.Group("http", "method", r.Method, "url", r.URL.String()))

	defer func(ctx context.Context) {
		if err != nil {
			log.ErrorContext(ctx, "user register failed", "error", err)
		} else {
			log.DebugContext(ctx, "media registered")
		}
	}(r.Context())

	// Parse form
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("parse form: %w", err)
	}

	username := r.FormValue("username")
	if username == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return ErrNoUsername
	}

	log = log.With(logging.Group("user", "username", username))

	password := r.FormValue("password")
	if password == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return ErrNoPassword
	}

	// Register user
	if err := ht.authSvc.RegisterUser(r.Context(), username, password); err != nil {
		if errors.Is(err, domain.ErrUserAlreadyExists) {
			http.Error(w, http.StatusText(http.StatusConflict), http.StatusConflict)
		} else {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		return fmt.Errorf("register user: %w", err)
	}

	return nil
}

// HandleLogin processes user login requests.
// Expects form parameters: username, password
// Returns an auth token on successful login.
func (ht *HTTPTransport) HandleLogin(w http.ResponseWriter, r *http.Request) {
	_ = ht.handleLogin(w, r)
}

func (ht *HTTPTransport) handleLogin(w http.ResponseWriter, r *http.Request) (err error) {
	log := ht.log.With(logging.Group("http", "method", r.Method, "url", r.URL.String()))

	defer func(ctx context.Context) {
		if err != nil {
			log.ErrorContext(ctx, "user login failed", "error", err)
		} else {
			log.DebugContext(ctx, "user logged in")
		}
	}(r.Context())

	// Parse form
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("parse form: %w", err)
	}

	username := r.FormValue("username")
	if username == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return ErrNoUsername
	}

	log = log.With(logging.Group("user", "username", username))

	password := r.FormValue("password")
	if password == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return ErrNoPassword
	}

	// Login user
	token, err := ht.authSvc.Login(r.Context(), username, password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		} else {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		return fmt.Errorf("login user: %w", err)
	}

	// Return token
	if err := json.NewEncoder(w).Encode(domain.AuthTokenResponse{Token: token}); err != nil {
		return fmt.Errorf("encode response: %w", err)
	}

	return nil
}

// HandleValidate processes token validation requests.
// Expects the token in the Authorization header with Bearer scheme.
// Returns the username associated with the token if valid.
func (ht *HTTPTransport) HandleValidate(w http.ResponseWriter, r *http.Request) {
	_ = ht.handleValidate(w, r)
}

func (ht *HTTPTransport) handleValidate(w http.ResponseWriter, r *http.Request) (err error) {
	log := ht.log.With(logging.Group("http", "method", r.Method, "url", r.URL.String()))

	defer func(ctx context.Context) {
		if err != nil {
			log.ErrorContext(ctx, "user token validation failed", "error", err)
		} else {
			log.DebugContext(ctx, "user token validated")
		}
	}(r.Context())

	// Parse headers
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return domain.ErrNoAuthToken
	}

	tokenString, _ := strings.CutPrefix(authHeader, "Bearer")
	tokenString = strings.TrimSpace(tokenString)

	// Validate token
	token, err := ht.authSvc.ValidateToken(r.Context(), tokenString)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)

		return fmt.Errorf("validate token: %w", err)
	}

	log = log.With(logging.Group("token",
		"username", token.Username,
		"exp", time.Unix(token.ExpiresAt, 0).UTC().Format(time.RFC3339),
		"iat", time.Unix(token.IssuedAt, 0).UTC().Format(time.RFC3339),
	))

	// Return username
	if _, err := w.Write([]byte(token.Username)); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}
