package mediasvc

import (
	"context"

	"github.com/mkrupp/homecase-michael/internal/domain"
)

// MediaService defines the interface for managing media objects.
type MediaService interface {
	// Lock acquires an exclusive lock for the specified media.
	// Returns a function that must be called to release the lock, and any error encountered.
	// The returned function must be called in a deferred statement to ensure the lock is released.
	Lock(ctx context.Context, mediaID domain.MediaID) (func(), error)

	// Store persists the given media object.
	// Returns an error if the operation fails or if the media exceeds configured size limits.
	Store(ctx context.Context, media domain.Media) error

	// Delete removes the media with the specified ID.
	// Returns whether the media was found and deleted, the ID of the deleted media,
	// and any error encountered during the operation.
	Delete(ctx context.Context, mediaID domain.MediaID) (bool, domain.MediaID, error)

	// Fetch retrieves the media with the specified ID.
	// Returns the media object if found, or an error if not found or if the operation fails.
	Fetch(ctx context.Context, mediaID domain.MediaID) (domain.Media, error)

	// MaxSize returns the maximum allowed file size for uploaded media in bytes.
	MaxSize() int64
}
