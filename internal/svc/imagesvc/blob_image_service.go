package imagesvc

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mkrupp/homecase-michael/internal/domain"
	"github.com/mkrupp/homecase-michael/internal/infra/logging"
	"github.com/mkrupp/homecase-michael/internal/repo/blob"
	"github.com/mkrupp/homecase-michael/internal/svc/authsvc/authclient"
	"github.com/mkrupp/homecase-michael/internal/svc/mediasvc"
)

// BlobImageService implements ImageService interface using blob storage.
// It uses a MediaService for base functionality and adds image-specific features
// like resizing and caching of resized images.
type BlobImageService struct {
	cacheRepo  blob.Repository
	mediaSvc   mediasvc.MediaService
	authClient authclient.AuthClient
	cfg        ImageConfig
	log        logging.Logger
}

var _ ImageService = (*BlobImageService)(nil)

// NewBlobImageService creates a new BlobImageService with the given configuration.
// It initializes a cache repository for storing resized images and requires:
// - A blob repository factory for creating the cache storage
// - A MediaService for handling basic media operations
// - An AuthClient for authentication
// Returns an error if repository initialization fails.
func NewBlobImageService(
	ctx context.Context,
	repoFactory blob.RepositoryFactory,
	mediaSvc mediasvc.MediaService,
	authClient authclient.AuthClient,
	cfg ImageConfig,
) (*BlobImageService, error) {
	cacheRepo, err := repoFactory(ctx, "cache", "bin")
	if err != nil {
		return nil, fmt.Errorf("new data repository: %w", err)
	}

	return &BlobImageService{
		cacheRepo:  cacheRepo,
		mediaSvc:   mediaSvc,
		authClient: authClient,
		cfg:        cfg,
		log:        logging.GetLogger("svc.imagesvc.blob_image_service"),
	}, nil
}

// Lock implements ImageService.Lock by delegating to the underlying MediaService.
func (imageSvc BlobImageService) Lock(ctx context.Context, imageID domain.MediaID) (func(), error) {
	//nolint:wrapcheck
	return imageSvc.mediaSvc.Lock(ctx, imageID)
}

// Store implements ImageService.Store by delegating to the underlying MediaService.
func (imageSvc BlobImageService) Store(ctx context.Context, image domain.Media) error {
	if _, _, err := imageSvc.CheckUploadConstraints(
		image.Meta().Filename,
		int64(len(image.Bytes())),
		image.Bytes(),
	); err != nil {
		return fmt.Errorf("check upload constraints: %w", err)
	}

	//nolint:wrapcheck
	return imageSvc.mediaSvc.Store(ctx, image)
}

// Delete implements ImageService.Delete and additionally handles cleanup of cached resized images
// when the original image is deleted.
func (imageSvc BlobImageService) Delete(ctx context.Context, imageID domain.MediaID) (err error) {
	log := imageSvc.log.With(logging.Group("image", "id", imageID))

	defer func() {
		if err != nil {
			log.ErrorContext(ctx, "image delete failed", "error", err)
		} else {
			log.DebugContext(ctx, "image deleted")
		}
	}()

	// Delete image
	pruned, dataID, err := imageSvc.mediaSvc.Delete(ctx, imageID)
	if err != nil {
		return fmt.Errorf("delete media: %w", err)
	}

	log = log.With(logging.Group("image", "pruned", pruned))

	// Delete caches if image was pruned
	if pruned {
		unlock, err := imageSvc.cacheRepo.Lock(ctx, dataID, true)
		if err != nil {
			return fmt.Errorf("lock cache: %w", err)
		}
		defer unlock()

		if err := imageSvc.cacheRepo.DeleteAll(ctx, dataID, "_*"); err != nil {
			return fmt.Errorf("delete cache: %w", err)
		}
	}

	return nil
}

// Fetch implements ImageService.Fetch with support for image resizing.
// If width is non-zero, returns a resized version of the image, using cached version if available.
// If width is zero, returns the original image.
func (imageSvc BlobImageService) Fetch(
	ctx context.Context,
	imageID domain.MediaID,
	width int,
) (image domain.Media, err error) {
	log := imageSvc.log.With(logging.Group("image", "id", imageID))

	defer func() {
		if err != nil {
			log.ErrorContext(ctx, "image fetch failed", "error", err)
		} else {
			log.DebugContext(ctx, "image fetched")
		}
	}()

	// Fetch image
	image, err = imageSvc.mediaSvc.Fetch(ctx, imageID)
	if err != nil {
		return domain.Media{}, fmt.Errorf("fetch media: %w", err)
	}

	if width == 0 {
		// Return original image
		return image, nil
	}

	// Try serve from cache
	cacheID := domain.BlobID(fmt.Sprintf("%s_%d", image.Hash(), width))

	unlock, err := imageSvc.cacheRepo.Lock(ctx, cacheID, false)
	if err != nil {
		return domain.Media{}, fmt.Errorf("lock cache: %w", err)
	}
	defer unlock()

	if imageSvc.cacheRepo.Exists(ctx, cacheID) {
		cacheBlob, err := imageSvc.cacheRepo.Fetch(ctx, cacheID)
		if err != nil {
			return domain.Media{}, fmt.Errorf("fetch cache: %w", err)
		}

		log = log.With(logging.Group("image", "cached", true))

		return domain.NewMedia(cacheBlob.Bytes(), image.Meta()), nil
	}

	// Resize image
	resized, err := imageSvc.resizeImage(ctx, image.Bytes(), image.MIMEType(), width)
	if err != nil {
		return domain.Media{}, fmt.Errorf("resize image: %w", err)
	}

	resizedMedia := domain.NewMedia(resized, image.Meta())

	// Update cache
	cacheBlob := domain.NewBlob(cacheID, resizedMedia.Bytes())

	if err := imageSvc.cacheRepo.Store(ctx, cacheBlob); err != nil {
		return domain.Media{}, fmt.Errorf("store: %w", err)
	}

	return resizedMedia, nil
}

func (imageSvc BlobImageService) MaxSize() int64 {
	return imageSvc.mediaSvc.MaxSize()
}

func (imageSvc BlobImageService) CheckUploadConstraints(
	filename string,
	size int64,
	image []byte,
) (string, bool, error) {
	if size > imageSvc.MaxSize() {
		return "", false, domain.ErrImageTooLarge
	}

	filenameExt := strings.ToLower(filepath.Ext(filename))

	imageType, ok := imageExtTypes[filenameExt]
	if !ok {
		return "", false, fmt.Errorf("%w: %q", domain.ErrImageTypeNotSupported, filenameExt)
	}

	if image == nil {
		return "", true, nil
	}

	for _, header := range imageExtHeaders[imageType] {
		if bytes.HasPrefix(image, []byte(header)) {
			return imageType, true, nil
		}
	}

	return "", false, fmt.Errorf("%w: %q", domain.ErrImageTypeMismatch, filenameExt)
}

func (imageSvc BlobImageService) resizeImage(
	ctx context.Context,
	data []byte,
	ctype string,
	width int,
) (resized []byte, err error) {
	log := imageSvc.log.With(logging.Group("image",
		"type", ctype,
		logging.Group("target", "width", width),
	))

	defer func() {
		if err != nil {
			log.ErrorContext(ctx, "image resize failed", "error", err)
		} else {
			log.DebugContext(ctx, "image resized")
		}
	}()

	return resizeImage(data, ctype, width, imageSvc.cfg.Interpolator)
}
