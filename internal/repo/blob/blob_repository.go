package blob

import (
	"context"

	"github.com/mkrupp/homecase-michael/internal/domain"
)

// Repository defines the interface for blob storage operations.
type Repository interface {
	// Lock acquires a lock on the blob with the given ID.
	// If exclusive is true, acquires a write lock, otherwise a read lock.
	// Returns a function to release the lock, and any error encountered.
	Lock(ctx context.Context, id domain.BlobID, exclusive bool) (func(), error)

	// Exists checks if a blob with the given ID exists.
	Exists(ctx context.Context, id domain.BlobID) bool

	// Store persists a blob in the repository.
	// Returns an error if the operation fails.
	Store(ctx context.Context, blob *domain.Blob) error

	// Fetch retrieves a blob by its ID.
	// Returns the blob if found, or an error if not found or if retrieval fails.
	Fetch(ctx context.Context, id domain.BlobID) (*domain.Blob, error)

	// Delete removes a blob with the given ID.
	// Returns an error if the blob doesn't exist or if deletion fails.
	Delete(ctx context.Context, id domain.BlobID) error

	// DeleteAll removes all blobs matching the given ID prefix and pattern.
	// Returns an error if any deletion operation fails.
	DeleteAll(ctx context.Context, id domain.BlobID, pattern string) error
}

// RepositoryFactory is a function that creates a new Repository instance.
// Parameters:
// - name: subdirectory name for the repository
// - ext: file extension for stored blobs
// Returns an error if initialization fails.
type RepositoryFactory func(
	ctx context.Context,
	name string,
	ext string,
) (Repository, error)
