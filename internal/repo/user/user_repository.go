package user

import (
	"context"

	"github.com/mkrupp/homecase-michael/internal/domain"
)

// Repository defines the interface for user data persistence.
type Repository interface {
	// CreateUser adds a new user to the repository.
	// Returns ErrUserAlreadyExists if the username is already taken.
	CreateUser(ctx context.Context, username string, passwordHash []byte) error

	// GetUserByUsername retrieves a user by their username.
	// Returns the user object and true if found, or nil and false if not found.
	// Returns an error if the operation fails.
	GetUserByUsername(ctx context.Context, username string) (*domain.User, bool, error)

	// Close releases any resources held by the repository.
	// Returns an error if cleanup fails.
	Close() error
}

// RepositoryFactory is a function that creates a new Repository instance.
// Returns an error if initialization fails.
type RepositoryFactory func() (Repository, error)
