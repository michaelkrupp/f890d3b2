package authsvc

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mkrupp/homecase-michael/internal/domain"
	"github.com/mkrupp/homecase-michael/internal/infra/logging"
	"github.com/mkrupp/homecase-michael/internal/repo/user"
)

// AuthConfig contains configuration parameters for the authentication service.
type AuthConfig struct {
	// SigningKeyFile is the path to the RSA private key file
	SigningKeyFile string `env:"SIGNING_KEY_FILE" default:"var/storage/authsvc.key"`

	// TokenDuration is the validity duration of auth tokens in seconds
	TokenDuration int64 `env:"TOKEN_DURATION" default:"3600"` // 1h
}

// AuthService provides authentication and user management functionality.
// It handles user registration, login, and token validation.
type AuthService struct {
	Config     AuthConfig
	UserRepo   user.Repository
	Log        logging.Logger
	SigningKey *rsa.PrivateKey
}

// NewAuthService creates a new AuthService with the given user repository factory and configuration.
// Returns an error if the signing key cannot be loaded or the user repository cannot be created.
func NewAuthService(repoFactory user.RepositoryFactory, cfg AuthConfig) (*AuthService, error) {
	log := logging.GetLogger("svc.authsvc.auth_service")

	signingKey, err := GetPrivateKey(cfg.SigningKeyFile)
	if err != nil {
		return nil, fmt.Errorf("get private key: %w", err)
	}

	userRepo, err := repoFactory()
	if err != nil {
		return nil, fmt.Errorf("new user repo: %w", err)
	}

	return &AuthService{
		Config:     cfg,
		UserRepo:   userRepo,
		Log:        log,
		SigningKey: signingKey,
	}, nil
}

// RegisterUser creates a new user account with the given username and password.
// The password is hashed before storage.
// Returns an error if the username is already taken or if creation fails.
func (s *AuthService) RegisterUser(ctx context.Context, username, password string) (err error) {
	log := s.Log.With(logging.Group("user", "username", username))

	defer func() {
		if err != nil {
			log.ErrorContext(ctx, "register user failed", "error", err)
		} else {
			log.DebugContext(ctx, "user registered")
		}
	}()

	hasher := sha256.New()
	hasher.Write([]byte(password))
	passwordHash := hasher.Sum(nil)

	if err := s.UserRepo.CreateUser(ctx, username, passwordHash); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	return nil
}

// Login authenticates a user and generates a signed JWT token.
// Returns the encoded token string or an error if authentication fails.
func (s *AuthService) Login(ctx context.Context, username, password string) (_ string, err error) {
	log := s.Log

	defer func() {
		if err != nil {
			log.ErrorContext(ctx, "login failed", "error", err)
		} else {
			log.DebugContext(ctx, "login successful")
		}
	}()

	// Authenticate user
	user, ok, err := s.UserRepo.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return "", errors.Join(domain.ErrInvalidCredentials, err)
		} else {
			return "", fmt.Errorf("get user: %w", err)
		}
	} else if !ok {
		return "", domain.ErrInvalidCredentials
	}

	hasher := sha256.New()
	hasher.Write([]byte(password))

	if !hmac.Equal(hasher.Sum(nil), user.PasswordHash) {
		return "", domain.ErrInvalidCredentials
	}

	// Generate token
	now := time.Now()
	expiry := now.Add(time.Duration(s.Config.TokenDuration * int64(time.Second)))
	token := domain.AuthToken{
		Username:  username,
		IssuedAt:  now.Unix(),
		ExpiresAt: expiry.Unix(),
	}

	log = log.With(logging.Group("token",
		"username", token.Username,
		"exp", expiry.UTC().Format(time.RFC3339),
		"iat", now.UTC().Format(time.RFC3339),
	))

	// Serialize token
	tokenBytes, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("marshal token: %w", err)
	}

	// Create signature
	hashed := sha256.Sum256(tokenBytes)

	signature, err := rsa.SignPSS(rand.Reader, s.SigningKey, crypto.SHA256, hashed[:], nil)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	// Encode combined token + signature
	return base64.URLEncoding.EncodeToString(append(tokenBytes, signature...)), nil
}

// ValidateToken verifies a JWT token's signature and expiration.
// Returns the decoded token if valid, or an error if validation fails.
func (s *AuthService) ValidateToken(ctx context.Context, tokenString string) (token domain.AuthToken, err error) {
	log := s.Log

	defer func() {
		if err != nil {
			log.ErrorContext(ctx, "validate token failed", "error", err)
		} else {
			log.DebugContext(ctx, "token validated")
		}
	}()

	token, err = ValidateToken(ctx, tokenString, &s.SigningKey.PublicKey)
	if err != nil {
		return domain.AuthToken{}, fmt.Errorf("validate token: %w", err)
	}

	log = log.With(logging.Group("token",
		"username", token.Username,
		"exp", time.Unix(token.ExpiresAt, 0).UTC().Format(time.RFC3339),
		"iat", time.Unix(token.IssuedAt, 0).UTC().Format(time.RFC3339),
	))

	return token, nil
}

// Close releases resources held by the service, such as database connections.
// Returns an error if cleanup fails.
func (s *AuthService) Close() error {
	if err := s.UserRepo.Close(); err != nil {
		return fmt.Errorf("close user repo: %w", err)
	}

	return nil
}
