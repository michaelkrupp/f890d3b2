package authclient

import "context"

// AuthClient defines the interface for validating authentication tokens.
type AuthClient interface {
	// Validate checks if the given token is valid.
	// Returns the username associated with the token, whether the token is valid,
	// and any error encountered during validation.
	Validate(ctx context.Context, token string) (string, bool, error)
}
