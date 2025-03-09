package blob

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/mkrupp/homecase-michael/internal/domain"
	"github.com/mkrupp/homecase-michael/internal/infra/logging"
)

var (
	ErrBytesWrittenMismatch = errors.New("bytes written mismatch")
	ErrBytesReadMismatch    = errors.New("bytes read mismatch")
)

const (
	dirPrefixLength = 2 // 16^2 = 256 directories
	dirPrefixDepth  = 3 // 256^3 = 16,777,216 directories
	idMinLength     = dirPrefixDepth * dirPrefixLength
)

// FileSystemBlobRepositoryConfig holds configuration for the filesystem-based blob repository.
type FileSystemBlobRepositoryConfig struct {
	// Basedir is the root directory for blob storage
	Basedir string `env:"BASEDIR" default:"var/storage/blob"`
}

// FileSystemBlobRepositoryFactory creates a factory function that returns a new FileSystemRepository.
// The factory function implements the RepositoryFactory type.
func FileSystemBlobRepositoryFactory(cfg FileSystemBlobRepositoryConfig) RepositoryFactory {
	return func(
		ctx context.Context,
		subdir string,
		ext string,
	) (Repository, error) {
		return NewFileSystemBlobRepository(ctx, subdir, ext, cfg)
	}
}

// NewFileSystemBlobRepository creates a new FileSystemRepository with the given parameters:
// - subdir: subdirectory name for organizing blobs
// - ext: file extension for blob files
// - cfg: repository configuration
// Returns an error if initialization fails.
func NewFileSystemBlobRepository(
	ctx context.Context,
	subdir string,
	ext string,
	cfg FileSystemBlobRepositoryConfig,
) (*FileSystemRepository, error) {
	log := logging.GetLogger("repo.blob.filesystem_repository").With(
		logging.Group("repo",
			"basedir", cfg.Basedir,
			"subdir", subdir,
			"ext", ext,
		),
	)

	repo := &FileSystemRepository{
		subdir: subdir,
		ext:    ext,
		cfg:    cfg,
		log:    log,
		m:      new(sync.Mutex),
	}

	if err := repo.initStorage(ctx); err != nil {
		return nil, fmt.Errorf("init repo: %w", err)
	}

	return repo, nil
}

// FileSystemRepository implements Repository using the local filesystem.
// It organizes blobs in a directory hierarchy to improve performance with large numbers of files.
type FileSystemRepository struct {
	subdir string
	ext    string
	cfg    FileSystemBlobRepositoryConfig
	log    logging.Logger
	m      *sync.Mutex
}

var _ Repository = (*FileSystemRepository)(nil)

func (fsRepo *FileSystemRepository) Lock(ctx context.Context, id domain.BlobID, exclusive bool) (func(), error) {
	filename := fsRepo.GetFilename(id)

	mode := syscall.LOCK_SH
	if exclusive {
		mode = syscall.LOCK_EX
	}

	release, err := fsRepo.flock(ctx, filename, mode)
	if err != nil {
		return nil, fmt.Errorf("flock: %w", err)
	}

	return release, nil
}

func (fsRepo *FileSystemRepository) Exists(ctx context.Context, id domain.BlobID) bool {
	return fsRepo.blobExists(id)
}

func (fsRepo *FileSystemRepository) Delete(ctx context.Context, id domain.BlobID) error {
	if err := fsRepo.deleteBlob(ctx, id); err != nil {
		return fmt.Errorf("delete blob: %w", err)
	}

	return nil
}

func (fsRepo *FileSystemRepository) DeleteAll(ctx context.Context, id domain.BlobID, pattern string) error {
	if err := fsRepo.deleteBlobPattern(ctx, id, pattern); err != nil {
		return fmt.Errorf("delete blob pattern: %w", err)
	}

	return nil
}

func (fsRepo *FileSystemRepository) Fetch(ctx context.Context, id domain.BlobID) (*domain.Blob, error) {
	blob, err := fsRepo.fetchBlob(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetch blob: %w", err)
	}

	return blob, nil
}

func (fsRepo *FileSystemRepository) Store(ctx context.Context, blob *domain.Blob) error {
	if err := fsRepo.storeBlob(ctx, blob); err != nil {
		return fmt.Errorf("store blob: %w", err)
	}

	return nil
}

func (fsRepo *FileSystemRepository) initStorage(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			fsRepo.log.ErrorContext(ctx, "init storage failed", "error", err)
		} else {
			fsRepo.log.DebugContext(ctx, "init storage")
		}
	}()

	if err := os.MkdirAll(fsRepo.cfg.Basedir, 0o755); err != nil {
		fsRepo.log.ErrorContext(ctx, "init storage", "error", err)

		return fmt.Errorf("mkdir all: %w", err)
	}

	return nil
}

func (fsRepo *FileSystemRepository) getBasename(id domain.BlobID) string {
	// Pad the id with zeros to the left to make it fit ith the directory structure
	basename := strings.ReplaceAll(string(id), "/", "")
	basename = strings.ReplaceAll(fmt.Sprintf("%*s", idMinLength, basename), " ", "0")

	// Split the filename into dirPrefixDepth chunks of dirPrefixLength characters
	// and create a directory structure like this:
	//   5f/56/69/2f/5f56692f0df9ff68607abdb054943ed86bcee7c9f2a2d01fdcb27032f70f3fe9.bin
	var prefixes []string
	for i := 0; i < dirPrefixLength*dirPrefixDepth && i < len(basename)-dirPrefixLength; i += dirPrefixLength {
		prefixes = append(prefixes, basename[i:i+dirPrefixLength])
	}

	return filepath.Join(append(append([]string{fsRepo.cfg.Basedir, fsRepo.subdir}, prefixes...), basename)...)
}

func (fsRepo *FileSystemRepository) getFilenames(id domain.BlobID, pattern string) (filenames []string, err error) {
	basename := fsRepo.getBasename(id)
	pattern = fmt.Sprintf("%s%s.%s", basename, pattern, fsRepo.ext)

	err = filepath.Walk(filepath.Dir(basename), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		if matched, err := filepath.Match(pattern, path); err != nil {
			return fmt.Errorf("match: %w", err)
		} else if !matched {
			return nil
		}

		filenames = append(filenames, path)

		return nil
	})

	return
}

// GetFilename returns the full filesystem path for a blob with the given ID.
func (fsRepo *FileSystemRepository) GetFilename(id domain.BlobID) string {
	return fmt.Sprintf("%s.%s", fsRepo.getBasename(id), fsRepo.ext)
}

func (fsRepo *FileSystemRepository) flock(ctx context.Context, filename string, mode int) (release func(), err error) {
	lockfile := filename + ".lock"
	log := fsRepo.log.With(logging.Group("blob", "lockfile", lockfile))

	defer func() {
		if err != nil {
			log.ErrorContext(ctx, "lock failed", "error", err)
		} else {
			log.DebugContext(ctx, "lock acquired")
		}
	}()

	if err := os.MkdirAll(filepath.Dir(lockfile), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir all: %w", err)
	}

	file, err := os.OpenFile(lockfile, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	err = syscall.Flock(int(file.Fd()), mode)
	if err != nil {
		_ = os.Remove(lockfile)
		_ = file.Close()

		return nil, fmt.Errorf("flock: %w", err)
	}

	return func() {
		_ = os.Remove(lockfile)
		_ = file.Close()
		_ = syscall.Flock(int(file.Fd()), mode)

		log.DebugContext(ctx, "lock released")
	}, nil
}

func (fsRepo *FileSystemRepository) blobExists(id domain.BlobID) bool {
	filename := fsRepo.GetFilename(id)
	_, err := os.Stat(filename)

	return err == nil
}

func (fsRepo *FileSystemRepository) storeBlob(ctx context.Context, blob *domain.Blob) (err error) {
	filename := fsRepo.GetFilename(blob.ID)

	defer func() {
		log := fsRepo.log.With(logging.Group("blob", "id", blob.ID, "filename", filename))
		if err != nil {
			log.ErrorContext(ctx, "blob store failed", "error", err)
		} else {
			log.DebugContext(ctx, "blob stored", "size", blob.Size())
		}
	}()

	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return fmt.Errorf("mkdir all: %w", err)
	}

	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer file.Close()

	if err := file.Truncate(blob.Size()); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}

	if bytes, err := blob.WriteTo(file); err != nil {
		return fmt.Errorf("write: %w", err)
	} else if err := file.Sync(); err != nil {
		return fmt.Errorf("sync: %w", err)
	} else if info, err := file.Stat(); err != nil {
		return fmt.Errorf("stat: %w", err)
	} else if bytes != info.Size() || bytes != blob.Size() {
		return fmt.Errorf("%w: expected %d, got %d", ErrBytesWrittenMismatch, blob.Size(), bytes)
	}

	return nil
}

func (fsRepo *FileSystemRepository) fetchBlob(
	ctx context.Context,
	blobID domain.BlobID,
) (blob *domain.Blob, err error) {
	filename := fsRepo.GetFilename(blobID)

	defer func() {
		log := fsRepo.log.With(logging.Group("blob", "id", blobID, "filename", filename))
		if err != nil {
			log.ErrorContext(ctx, "blob fetch failed", "error", err)
		} else {
			log.DebugContext(ctx, "blob fetched")
		}
	}()

	file, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer file.Close()

	//nolint:exhaustruct
	data := &domain.Blob{ID: blobID}
	if n, err := data.ReadFrom(file); err != nil {
		return nil, fmt.Errorf("read: %w", err)
	} else if info, err := file.Stat(); err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	} else if n != info.Size() || n != data.Size() {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrBytesReadMismatch, info.Size(), n)
	}

	return data, nil
}

func (fsRepo *FileSystemRepository) deleteBlob(ctx context.Context, id domain.BlobID) (err error) {
	filename := fsRepo.GetFilename(id)

	defer func() {
		log := fsRepo.log.With(logging.Group("blob", "id", id, "filename", filename))
		if err != nil {
			log.ErrorContext(ctx, "blob delete failed", "error", err)
		} else {
			log.DebugContext(ctx, "blob deleted")
		}
	}()

	if err := os.Remove(filename); err != nil {
		return fmt.Errorf("remove: %w", err)
	}

	return nil
}

func (fsRepo *FileSystemRepository) deleteBlobPattern(
	ctx context.Context,
	blobID domain.BlobID,
	pattern string,
) (err error) {
	defer func() {
		log := fsRepo.log.With(logging.Group("blob", "id", blobID, "pattern", pattern))
		if err != nil {
			log.ErrorContext(ctx, "blob delete pattern failed", "error", err)
		} else {
			log.DebugContext(ctx, "blob pattern deleted")
		}
	}()

	filenames, err := fsRepo.getFilenames(blobID, pattern)
	if err != nil {
		return fmt.Errorf("get filenames: %w", err)
	}

	for _, filename := range filenames {
		if err := os.Remove(filename); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("remove: %w", err)
			}
		}
	}

	return nil
}
