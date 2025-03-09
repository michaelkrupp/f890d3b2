package domain

import "errors"

var (
	// ErrUserAlreadyExists is returned when trying to create a user with an existing username.
	ErrUserAlreadyExists = errors.New("user already exists")
	// ErrUserNotFound is returned when looking up a non-existent user.
	ErrUserNotFound = errors.New("user not found")
	// ErrInvalidCredentials is returned when the username/password combination is incorrect.
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// User represents an authenticated user in the system.
type User struct {
	ID           int64  // Unique identifier
	Username     string // Login username
	PasswordHash []byte // Hashed password
	CreatedAt    int64  // Unix timestamp of account creation
}
