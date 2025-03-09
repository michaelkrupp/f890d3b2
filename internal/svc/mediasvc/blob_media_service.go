package mediasvc

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/mkrupp/homecase-michael/internal/domain"
	context_ "github.com/mkrupp/homecase-michael/internal/infra/context"
	"github.com/mkrupp/homecase-michael/internal/infra/logging"
	"github.com/mkrupp/homecase-michael/internal/repo/blob"
)

// BlobMediaService implements MediaService interface using blob storage.
// It manages media data and metadata in separate blob repositories and maintains
// backreferences to efficiently de-duplicate shared content.
type BlobMediaService struct {
	dataRepo    blob.Repository
	metaRepo    blob.Repository
	backrefRepo blob.Repository
	cfg         MediaConfig
	log         logging.Logger
}

var _ MediaService = (*BlobMediaService)(nil)

// NewBlobMediaService creates a new BlobMediaService with the given configuration.
// It initializes three blob repositories:
// - data: for storing actual media content
// - meta: for storing media metadata
// - backref: for managing references to shared content
// Returns an error if any repository initialization fails.
func NewBlobMediaService(
	ctx context.Context,
	repoFactory blob.RepositoryFactory,
	cfg MediaConfig,
) (*BlobMediaService, error) {
	log := logging.GetLogger("svc.mediasvc.blob_media_service")

	dataRepo, err := repoFactory(ctx, "data", "bin")
	if err != nil {
		return nil, fmt.Errorf("new data repository: %w", err)
	}

	backrefRepo, err := repoFactory(ctx, "data", "txt")
	if err != nil {
		return nil, fmt.Errorf("new backref repository: %w", err)
	}

	metaRepo, err := repoFactory(ctx, "meta", "json")
	if err != nil {
		return nil, fmt.Errorf("new meta repository: %w", err)
	}

	return &BlobMediaService{
		dataRepo:    dataRepo,
		metaRepo:    metaRepo,
		backrefRepo: backrefRepo,
		cfg:         cfg,
		log:         log,
	}, nil
}

// MaxSize implements MediaService.MaxSize.
func (mediaSvc BlobMediaService) MaxSize() int64 {
	return mediaSvc.cfg.MaxSize
}

// Lock implements MediaService.Lock.
func (mediaSvc BlobMediaService) Lock(ctx context.Context, mediaID domain.MediaID) (unlock func(), err error) {
	log := mediaSvc.log.With(logging.Group("media", "id", mediaID))

	defer func() {
		if err != nil {
			log.ErrorContext(ctx, "media lock failed", "error", err)
		} else {
			log.DebugContext(ctx, "media locked")
		}
	}()

	unlock, err = mediaSvc.metaRepo.Lock(ctx, mediaID, false)
	if err != nil {
		return func() {}, fmt.Errorf("lock meta: %w", err)
	}

	return func() {
		unlock()
		log.DebugContext(ctx, "media unlocked")
	}, nil
}

// Store implements MediaService.Store.
//
//nolint:cyclop
func (mediaSvc BlobMediaService) Store(
	ctx context.Context,
	media domain.Media,
) (err error) {
	log := mediaSvc.log.With(logging.Group("media",
		"id", media.ID(),
		"size", media.Size(),
		"type", media.MIMEType(),
	))

	defer func() {
		if err != nil {
			log.ErrorContext(ctx, "media store failed", "error", err)
		} else {
			log.DebugContext(ctx, "media stored")
		}
	}()

	if media.Size() > mediaSvc.cfg.MaxSize {
		return fmt.Errorf("%w: %d exceeds %d", domain.ErrMediaTooLarge, media.Size(), mediaSvc.cfg.MaxSize)
	}

	// Lock meta blob
	metaBlob, err := media.Meta().AsBlob()
	if err != nil {
		return fmt.Errorf("convert meta to blob: %w", err)
	}

	unlockMeta, err := mediaSvc.metaRepo.Lock(ctx, metaBlob.ID, true)
	if err != nil {
		return fmt.Errorf("lock meta: %w", err)
	}
	defer unlockMeta()

	// Lock data blob
	dataBlob := media.AsBlob()

	unlockData, err := mediaSvc.dataRepo.Lock(ctx, dataBlob.ID, true)
	if err != nil {
		return fmt.Errorf("lock data: %w", err)
	}
	defer unlockData()

	// Store data
	if !mediaSvc.dataRepo.Exists(ctx, dataBlob.ID) {
		if err := mediaSvc.dataRepo.Store(ctx, dataBlob); err != nil {
			return fmt.Errorf("store data: %w", err)
		}
	}

	// Store meta and add backrefs
	if !mediaSvc.metaRepo.Exists(ctx, metaBlob.ID) {
		if err := mediaSvc.metaRepo.Store(ctx, metaBlob); err != nil {
			return fmt.Errorf("store meta: %w", err)
		}

		if err := mediaSvc.addBackrefs(ctx, dataBlob.ID, metaBlob.ID); err != nil {
			return fmt.Errorf("add backrefs: %w", err)
		}
	}

	return nil
}

// Delete implements MediaService.Delete.
func (mediaSvc BlobMediaService) Delete(
	ctx context.Context,
	mediaID domain.MediaID,
) (prune bool, dataID domain.BlobID, err error) {
	log := mediaSvc.log.With(logging.Group("media", "id", mediaID))

	defer func() {
		if err != nil {
			log.ErrorContext(ctx, "media delete failed", "error", err)
		} else {
			log.DebugContext(ctx, "media deleted")
		}
	}()

	// Lock meta blob
	unlockMeta, err := mediaSvc.metaRepo.Lock(ctx, mediaID, false)
	if err != nil {
		return false, domain.BlobID(""), fmt.Errorf("lock meta: %w", err)
	}
	defer unlockMeta()

	// Fetch meta
	mediaMeta, err := mediaSvc.fetchMeta(ctx, mediaID)
	if err != nil {
		return false, domain.BlobID(""), fmt.Errorf("fetch meta: %w", err)
	}

	dataID = domain.BlobID(mediaMeta.Hash)

	log = log.With(logging.Group("media",
		"dataID", dataID,
		"size", mediaMeta.Size,
		"type", mediaMeta.MIMEType,
		"filename", mediaMeta.Filename,
		"hash", mediaMeta.Hash,
		"owner", mediaMeta.Owner,
	))

	// Authorize access
	username, ok := context_.UsernameFromContext(ctx)
	if !ok || strings.Compare(username, mediaMeta.Owner) != 0 {
		return false, domain.BlobID(""), fmt.Errorf("%w: user %q is not owner %q",
			domain.ErrUnauthorized, username, mediaMeta.Owner)
	}

	// Lock data blob
	unlockData, err := mediaSvc.dataRepo.Lock(ctx, domain.BlobID(mediaMeta.Hash), true)
	if err != nil {
		return false, dataID, fmt.Errorf("lock data: %w", err)
	}
	defer unlockData()

	// Update backrefs or delete blob
	pruned, err := mediaSvc.pruneMedia(ctx, mediaMeta)
	if err != nil {
		return false, dataID, fmt.Errorf("prune media: %w", err)
	}

	// Delete meta
	if err := mediaSvc.metaRepo.Delete(ctx, mediaID); err != nil {
		return pruned, dataID, fmt.Errorf("delete meta: %w", err)
	}

	return pruned, dataID, nil
}

// Fetch implements MediaService.Fetch.
func (mediaSvc BlobMediaService) Fetch(
	ctx context.Context,
	mediaID domain.MediaID,
) (media domain.Media, err error) {
	log := mediaSvc.log.With(logging.Group("media",
		"id", mediaID,
	))

	defer func() {
		if err != nil {
			log.ErrorContext(ctx, "media fetch failed", "error", err)
		} else {
			log.DebugContext(ctx, "media fetched")
		}
	}()

	// Lock meta blob
	unlockMeta, err := mediaSvc.metaRepo.Lock(ctx, mediaID, false)
	if err != nil {
		return domain.Media{}, fmt.Errorf("lock meta: %w", err)
	}
	defer unlockMeta()

	// Fetch meta blob
	metaBlob, err := mediaSvc.metaRepo.Fetch(ctx, mediaID)
	if err != nil {
		return domain.Media{}, fmt.Errorf("fetch meta: %w", err)
	}

	mediaMeta, err := domain.NewMediaMetaFromBlob(metaBlob)
	if err != nil {
		return domain.Media{}, fmt.Errorf("convert meta blob: %w", err)
	}

	log = log.With(logging.Group("media",
		"size", mediaMeta.Size,
		"type", mediaMeta.MIMEType,
		"filename", mediaMeta.Filename,
		"hash", mediaMeta.Hash,
		"owner", mediaMeta.Owner,
	))

	// Authorize access
	username, ok := context_.UsernameFromContext(ctx)
	if !ok || strings.Compare(username, mediaMeta.Owner) != 0 {
		return domain.Media{}, fmt.Errorf("%w: user %q is not owner %q", domain.ErrUnauthorized, username, mediaMeta.Owner)
	}

	// Lock data blob
	unlockData, err := mediaSvc.dataRepo.Lock(ctx, domain.BlobID(mediaMeta.Hash), false)
	if err != nil {
		return domain.Media{}, fmt.Errorf("lock data: %w", err)
	}
	defer unlockData()

	// Fetch data blob
	dataBlob, err := mediaSvc.dataRepo.Fetch(ctx, domain.BlobID(mediaMeta.Hash))
	if err != nil {
		return domain.Media{}, fmt.Errorf("fetch data: %w", err)
	}

	log = log.With(logging.Group("media",
		"dataID", dataBlob.ID,
	))

	return domain.NewMedia(dataBlob.Bytes(), mediaMeta), nil
}

func (mediaSvc BlobMediaService) fetchMeta(
	ctx context.Context,
	mediaID domain.MediaID,
) (meta domain.MediaMeta, err error) {
	defer func() {
		log := mediaSvc.log.With(logging.Group("media", "id", meta.ID))

		if err != nil {
			log.ErrorContext(ctx, "media fetch-meta failed", "error", err)
		} else {
			log.DebugContext(ctx, "media meta fetched")
		}
	}()

	metaBlob, err := mediaSvc.metaRepo.Fetch(ctx, mediaID)
	if err != nil {
		return domain.MediaMeta{}, fmt.Errorf("fetch meta: %w", err)
	}

	mediaMeta, err := domain.NewMediaMetaFromBlob(metaBlob)
	if err != nil {
		return domain.MediaMeta{}, fmt.Errorf("convert meta blob: %w", err)
	}

	return mediaMeta, nil
}

func (mediaSvc BlobMediaService) fetchBackrefs(
	ctx context.Context,
	blobID domain.BlobID,
) (backrefs []domain.BlobID, err error) {
	defer func() {
		log := mediaSvc.log.With(logging.Group("media",
			"id", blobID,
			"backrefs", backrefs,
		))

		if err != nil {
			log.ErrorContext(ctx, "media fetch-backrefs failed", "error", err)
		} else {
			log.DebugContext(ctx, "media backrefs fetched")
		}
	}()

	if !mediaSvc.backrefRepo.Exists(ctx, blobID) {
		return backrefs, nil
	}

	backrefBlob, err := mediaSvc.backrefRepo.Fetch(ctx, blobID)
	if err != nil {
		return backrefs, fmt.Errorf("fetch backref: %w", err)
	}

	for _, id := range bytes.Split(backrefBlob.Bytes(), []byte("\n")) {
		backrefs = append(backrefs, domain.BlobID(id))
	}

	return backrefs, nil
}

func (mediaSvc BlobMediaService) storeBackrefs(
	ctx context.Context,
	blobID domain.BlobID,
	backrefs []domain.BlobID,
) (err error) {
	defer func() {
		log := mediaSvc.log.With(logging.Group("media",
			"id", blobID,
			"backrefs", backrefs,
		))

		if err != nil {
			log.ErrorContext(ctx, "media store-backrefs failed", "error", err)
		} else {
			log.DebugContext(ctx, "media backrefs stored")
		}
	}()

	backrefBytes := make([][]byte, len(backrefs))
	for i, id := range backrefs {
		backrefBytes[i] = []byte(id)
	}

	backrefBlob := domain.NewBlob(blobID, bytes.Join(backrefBytes, []byte("\n")))

	if err := mediaSvc.backrefRepo.Store(ctx, backrefBlob); err != nil {
		return fmt.Errorf("store backref: %w", err)
	}

	return nil
}

func (mediaSvc BlobMediaService) addBackrefs(
	ctx context.Context,
	dataID domain.BlobID,
	metaIDs ...domain.BlobID,
) (err error) {
	backrefs, err := mediaSvc.fetchBackrefs(ctx, dataID)
	if err != nil {
		return fmt.Errorf("fetch backrefs: %w", err)
	}

	backrefs = append(backrefs, metaIDs...)

	if err := mediaSvc.storeBackrefs(ctx, dataID, backrefs); err != nil {
		return fmt.Errorf("store backrefs: %w", err)
	}

	return nil
}

func (mediaSvc BlobMediaService) pruneMedia(ctx context.Context, mediaMeta domain.MediaMeta) (pruned bool, err error) {
	defer func() {
		log := mediaSvc.log.With(logging.Group("media", "id", mediaMeta.ID))

		if err != nil {
			log.ErrorContext(ctx, "media pruneing failed", "error", err)
		} else {
			log.DebugContext(ctx, "media pruned")
		}
	}()

	backrefs, err := mediaSvc.fetchBackrefs(ctx, domain.BlobID(mediaMeta.Hash))
	if err != nil {
		return false, fmt.Errorf("fetch backrefs: %w", err)
	}

	backrefs = slices.DeleteFunc(backrefs, func(v domain.BlobID) bool {
		return strings.Compare(string(v), string(mediaMeta.ID)) == 0
	})

	dataID := domain.BlobID(mediaMeta.Hash)

	if len(backrefs) == 0 {
		if err := mediaSvc.dataRepo.Delete(ctx, dataID); err != nil {
			return false, fmt.Errorf("delete data: %w", err)
		}

		if err := mediaSvc.backrefRepo.Delete(ctx, dataID); err != nil {
			return false, fmt.Errorf("delete backrefs: %w", err)
		}

		return true, nil
	}

	if err := mediaSvc.storeBackrefs(ctx, domain.BlobID(mediaMeta.Hash), backrefs); err != nil {
		return false, fmt.Errorf("store backrefs: %w", err)
	}

	return false, nil
}
