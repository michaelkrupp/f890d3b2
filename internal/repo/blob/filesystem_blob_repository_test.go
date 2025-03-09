//go:build integration || all

package blob_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mkrupp/homecase-michael/internal/domain"
	"github.com/mkrupp/homecase-michael/internal/infra/logging"

	. "github.com/mkrupp/homecase-michael/internal/repo/blob"
)

func setupFileSystemBlobTestRepo(t *testing.T) (repo *FileSystemRepository, tempDir string, cleanup func()) {
	t.Helper()

	logging.Configure(context.TODO(), logging.LoggerConfig{
		OutputHandle: os.Stderr,
		Level:        "debug",
	}, "test")

	// Create temp directory for tests
	tempDir = t.TempDir()

	cfg := FileSystemBlobRepositoryConfig{
		Basedir: tempDir,
	}

	repo, err := NewFileSystemBlobRepository(context.TODO(), "test", "bin", cfg)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	cleanup = func() {
		os.RemoveAll(tempDir)
	}

	return repo, tempDir, cleanup
}

func verifyFileSystemBlobContent(t *testing.T, path string, expectedContent []byte) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read stored file: %v", err)
	}

	if !bytes.Equal(expectedContent, content) {
		t.Errorf("content mismatch\nwant: %s\ngot:  %s", expectedContent, content)
	}
}

func TestFileSystemBlobRepository_Store(t *testing.T) {
	t.Parallel()

	repo, _, cleanup := setupFileSystemBlobTestRepo(t)
	t.Cleanup(cleanup)

	tests := []struct {
		name     string
		blob     *domain.Blob
		wantBody []byte
	}{
		{
			name:     "handles new blob",
			blob:     &domain.Blob{ID: domain.BlobID("existingblob"), Body: []byte("original content")},
			wantBody: []byte("original content"),
		},
		{
			name:     "handles existing blob",
			blob:     &domain.Blob{ID: domain.BlobID("existingblob"), Body: []byte("new content")},
			wantBody: []byte("new content"),
		},
		{
			name:     "handles empty blob",
			blob:     &domain.Blob{ID: domain.BlobID("emptyblob"), Body: []byte("")},
			wantBody: []byte(""),
		},
		{
			name:     "handles large blob",
			blob:     &domain.Blob{ID: domain.BlobID("largeblob"), Body: make([]byte, 100*1024*1024)},
			wantBody: make([]byte, 100*1024*1024), // 100MB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Lock blob
			unlock, err := repo.Lock(context.TODO(), tt.blob.ID, true)
			if err != nil {
				t.Fatalf("failed to lock blob: %v", err)
			}
			t.Cleanup(unlock)

			// Store blob
			if err := repo.Store(context.TODO(), tt.blob); err != nil {
				t.Fatalf("failed to store blob: %v", err)
			}

			// Verify file existence and content
			storedPath := repo.GetFilename(tt.blob.ID)
			if _, err := os.Stat(storedPath); os.IsNotExist(err) {
				t.Error("expected file to exist, but it doesn't")
			}

			verifyFileSystemBlobContent(t, storedPath, tt.wantBody)
		})
	}
}

func TestFileSystemBlobRepository_Fetch(t *testing.T) {
	t.Parallel()

	repo, _, cleanup := setupFileSystemBlobTestRepo(t)
	t.Cleanup(cleanup)

	tests := []struct {
		name      string
		blob      *domain.Blob
		storeBlob bool
		wantErr   bool
	}{
		{
			name:      "handles existing blob",
			blob:      &domain.Blob{ID: domain.BlobID("existingblob"), Body: []byte("test content")},
			storeBlob: true,
			wantErr:   false,
		},
		{
			name:      "handles missing blob",
			blob:      &domain.Blob{ID: domain.BlobID("missingblob")},
			storeBlob: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.storeBlob {
				// Lock blob
				unlock, err := repo.Lock(context.TODO(), tt.blob.ID, true)
				if err != nil {
					t.Fatalf("failed to lock blob: %v", err)
				}
				t.Cleanup(unlock)

				// Store blob
				if err := repo.Store(context.TODO(), tt.blob); err != nil {
					t.Fatalf("failed to store blob: %v", err)
				}
			}

			// Fetch blob
			fetchedBlob, err := repo.Fetch(context.TODO(), tt.blob.ID)
			if (err != nil) != tt.wantErr {
				t.Errorf("unexpected error: %v", err)
			}

			if fetchedBlob != nil && tt.wantErr {
				t.Errorf("expected nil blob for missing blob")
			}
		})
	}
}

func TestFileSystemBlobRepository_Delete(t *testing.T) {
	t.Parallel()

	repo, tempDir, cleanup := setupFileSystemBlobTestRepo(t)
	t.Cleanup(cleanup)

	tests := []struct {
		name      string
		blob      *domain.Blob
		storeBlob bool
		wantErr   bool
	}{
		{
			name:      "handles existing blob",
			blob:      &domain.Blob{ID: domain.BlobID("existingblob"), Body: []byte("test content")},
			storeBlob: true,
			wantErr:   false,
		},
		{
			name:      "handles missing blob",
			blob:      &domain.Blob{ID: domain.BlobID("missingblob")},
			storeBlob: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.storeBlob {
				// Lock blob
				unlock, err := repo.Lock(context.TODO(), tt.blob.ID, true)
				if err != nil {
					t.Fatalf("failed to lock blob: %v", err)
				}
				t.Cleanup(unlock)

				// Store blob
				if err := repo.Store(context.TODO(), tt.blob); err != nil {
					t.Fatalf("failed to store blob: %v", err)
				}
			}

			// Try delete blob
			err := repo.Delete(context.TODO(), tt.blob.ID)
			if (err != nil) != tt.wantErr {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.wantErr {
				storedPath := filepath.Join(tempDir, string(tt.blob.ID)+".bin")
				if _, err := os.Stat(storedPath); !os.IsNotExist(err) {
					t.Error("expected file to be deleted, but it still exists")
				}
			}
		})
	}
}

func TestFileSystemBlobRepository_Lock(t *testing.T) {
	t.Parallel()

	repo, _, cleanup := setupFileSystemBlobTestRepo(t)
	t.Cleanup(cleanup)

	tests := []struct {
		name      string
		id        domain.BlobID
		exclusive bool
		parallel  bool // whether to try acquiring lock in parallel
		wantErr   bool
	}{
		{
			name:      "shared lock allows multiple readers",
			id:        "sharedlock",
			exclusive: false,
			parallel:  true,
			wantErr:   false,
		},
		// {
		// 	name:      "exclusive lock blocks other locks",
		// 	id:        "exclusivelock",
		// 	exclusive: true,
		// 	parallel:  true,
		// 	wantErr:   true,
		// },
		{
			name:      "can reacquire after release",
			id:        "relock",
			exclusive: true,
			parallel:  false,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Acquire first lock
			unlock1, err := repo.Lock(context.TODO(), tt.id, tt.exclusive)
			if err != nil {
				t.Fatalf("failed to acquire first lock: %v", err)
			}
			defer unlock1()

			if tt.parallel {
				// Try to acquire second lock
				_, err = repo.Lock(context.TODO(), tt.id, tt.exclusive)
				if (err != nil) != tt.wantErr {
					t.Errorf("unexpected error acquiring second lock: %v", err)
				}
			} else {
				// Release and reacquire
				unlock1()
				unlock2, err := repo.Lock(context.TODO(), tt.id, tt.exclusive)
				if (err != nil) != tt.wantErr {
					t.Errorf("unexpected error reacquiring lock: %v", err)
				}
				defer unlock2()
			}
		})
	}
}
