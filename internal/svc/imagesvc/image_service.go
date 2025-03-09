package imagesvc

import (
	"context"

	"github.com/mkrupp/homecase-michael/internal/domain"
)

// ImageService defines the interface for managing image objects.
type ImageService interface {
	// Lock acquires an exclusive lock for the specified image.
	// Returns a function that must be called to release the lock, and any error encountered.
	Lock(ctx context.Context, imageID domain.MediaID) (func(), error)

	// Store persists the given image.
	// Returns an error if the operation fails or if the image format is not supported.
	Store(ctx context.Context, image domain.Media) error

	// Delete removes the image with the specified ID.
	// Returns an error if the image was not found or if the operation fails.
	Delete(ctx context.Context, imageID domain.MediaID) error

	// Fetch retrieves and optionally resizes the image with the specified ID.
	// The width parameter controls the target width of the image, maintaining aspect ratio.
	// Returns the image object if found, or an error if not found or if the operation fails.
	Fetch(ctx context.Context, imageID domain.MediaID, width int) (domain.Media, error)

	// MaxSize returns the maximum allowed file size in bytes.
	MaxSize() int64

	// CheckUploadConstraints checks if the given file meets the upload constraints.
	// Returns true if the file is allowed to be uploaded, or an error if the constraints are not met.
	CheckUploadConstraints(filename string, size int64, image []byte) (string, bool, error)
}
