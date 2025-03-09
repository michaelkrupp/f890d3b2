package domain

import "errors"

var (
	// ErrNoAuthToken is returned when an authentication token is required but not provided.
	ErrNoAuthToken = errors.New("no auth token")
	// ErrInvalidAuthToken is returned when a token's signature is invalid or it has expired.
	ErrInvalidAuthToken = errors.New("invalid auth token")
	// ErrUnauthorized is returned when the authenticated user lacks permission.
	ErrUnauthorized = errors.New("unauthorized")
)

// AuthToken represents an authentication token with user information and validity period.
type AuthToken struct {
	Username  string `json:"username"`  // Identifier of the authenticated user
	IssuedAt  int64  `json:"issuedAt"`  // Unix timestamp when the token was created
	ExpiresAt int64  `json:"expiresAt"` // Unix timestamp when the token expires
}

// AuthTokenResponse represents a response containing an authentication token.
type AuthTokenResponse struct {
	Token string `json:"token"`
}
