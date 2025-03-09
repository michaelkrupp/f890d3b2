package mediasvc_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mkrupp/homecase-michael/internal/domain"
	context_ "github.com/mkrupp/homecase-michael/internal/infra/context"
	"github.com/mkrupp/homecase-michael/internal/repo/blob"
	"github.com/mkrupp/homecase-michael/internal/svc/mediasvc"
)

type mockRepository struct {
	blobs     map[domain.BlobID][]byte
	m         *sync.Mutex
	lockErr   error
	storeErr  error
	fetchErr  error
	deleteErr error
}

func (m *mockRepository) Lock(_ context.Context, _ domain.BlobID, _ bool) (func(), error) {
	if m.lockErr != nil {
		return nil, m.lockErr
	}
	return func() {}, nil
}

func (m *mockRepository) Store(_ context.Context, blob *domain.Blob) error {
	if m.storeErr != nil {
		return m.storeErr
	}
	m.m.Lock()
	defer m.m.Unlock()
	m.blobs[blob.ID] = blob.Body
	return nil
}

func (m *mockRepository) Fetch(_ context.Context, id domain.BlobID) (*domain.Blob, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	m.m.Lock()
	defer m.m.Unlock()
	data, exists := m.blobs[id]
	if !exists {
		return nil, errors.New("blob not found")
	}
	return domain.NewBlob(id, data), nil
}

func (m *mockRepository) Delete(_ context.Context, id domain.BlobID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.blobs, id)
	return nil
}

func (m *mockRepository) DeleteAll(_ context.Context, _ domain.BlobID, _ string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	return nil
}

func (m *mockRepository) Exists(_ context.Context, id domain.BlobID) bool {
	m.m.Lock()
	defer m.m.Unlock()
	_, exists := m.blobs[id]
	return exists
}

func newMockRepo() *mockRepository {
	return &mockRepository{
		blobs: make(map[domain.BlobID][]byte),
		m:     &sync.Mutex{},
	}
}

func setupMediaService(t *testing.T) (*mediasvc.BlobMediaService, *mockRepository, *mockRepository, *mockRepository) {
	t.Helper()

	dataRepo := newMockRepo()
	metaRepo := newMockRepo()
	backrefRepo := newMockRepo()

	factory := func(ctx context.Context, name string, ext string) (blob.Repository, error) {
		switch {
		case name == "data" && ext == "bin":
			return dataRepo, nil
		case name == "meta" && ext == "json":
			return metaRepo, nil
		default:
			return backrefRepo, nil
		}
	}

	svc, err := mediasvc.NewBlobMediaService(context.Background(), factory, mediasvc.MediaConfig{
		MaxSize: 1024 * 1024, // 1MB
	})
	if err != nil {
		t.Fatalf("failed to create media service: %v", err)
	}

	return svc, dataRepo, metaRepo, backrefRepo
}

func TestBlobMediaService_Store(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		media      domain.Media
		setupMocks func(*mockRepository, *mockRepository, *mockRepository)
		wantErr    bool
	}{
		{
			name:       "stores new media successfully",
			media:      domain.NewMedia([]byte("test data"), domain.MediaMeta{Filename: "test.txt"}),
			setupMocks: func(d, m, b *mockRepository) {},
			wantErr:    false,
		},
		{
			name:  "handles data store error",
			media: domain.NewMedia([]byte("test data"), domain.MediaMeta{Filename: "test.txt"}),
			setupMocks: func(d, m, b *mockRepository) {
				d.storeErr = errors.New("store failed")
			},
			wantErr: true,
		},
		{
			name:       "media too large",
			media:      domain.NewMedia(make([]byte, 2*1024*1024), domain.MediaMeta{Filename: "large.txt"}),
			setupMocks: func(d, m, b *mockRepository) {},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create fresh repositories for each test
			svc, dataRepo, metaRepo, backrefRepo := setupMediaService(t)

			// Setup mocks for this specific test
			tt.setupMocks(dataRepo, metaRepo, backrefRepo)

			ctx := context_.WithUsername(context.Background(), "testuser")
			err := svc.Store(ctx, tt.media)

			if (err != nil) != tt.wantErr {
				t.Errorf("Store() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify data was stored
				if !dataRepo.Exists(ctx, domain.BlobID(tt.media.Hash())) {
					t.Error("data blob was not stored")
				}
				// Verify metadata was stored
				if !metaRepo.Exists(ctx, tt.media.ID()) {
					t.Error("meta blob was not stored")
				}
			}
		})
	}
}

func TestBlobMediaService_Fetch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupMedia func(svc *mediasvc.BlobMediaService) domain.Media
		username   string
		dataErr    error
		metaErr    error
		wantErr    bool
	}{
		{
			name: "fetches existing media",
			setupMedia: func(svc *mediasvc.BlobMediaService) domain.Media {
				media := domain.NewMedia([]byte("test data"), domain.MediaMeta{
					Filename: "test.txt",
					MIMEType: "text/plain",
					Owner:    "testuser",
				})
				ctx := context_.WithUsername(context.Background(), "testuser")
				if err := svc.Store(ctx, media); err != nil {
					t.Fatalf("failed to store test media: %v", err)
				}
				return media
			},
			username: "testuser",
			wantErr:  false,
		},
		{
			name: "unauthorized user",
			setupMedia: func(svc *mediasvc.BlobMediaService) domain.Media {
				media := domain.NewMedia([]byte("test data"), domain.MediaMeta{
					Filename: "test.txt",
					Owner:    "testuser",
				})
				ctx := context_.WithUsername(context.Background(), "testuser")
				if err := svc.Store(ctx, media); err != nil {
					t.Fatalf("failed to store test media: %v", err)
				}
				return media
			},
			username: "otheruser",
			wantErr:  true,
		},
		{
			name: "non-existent media",
			setupMedia: func(svc *mediasvc.BlobMediaService) domain.Media {
				return domain.NewMedia(nil, domain.MediaMeta{ID: "nonexistent"})
			},
			username: "testuser",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create fresh service and repositories for each test
			svc, dataRepo, metaRepo, _ := setupMediaService(t)

			// Setup test data
			testMedia := tt.setupMedia(svc)

			// Setup error conditions
			dataRepo.fetchErr = tt.dataErr
			metaRepo.fetchErr = tt.metaErr

			// Execute test
			ctx := context_.WithUsername(context.Background(), tt.username)
			media, err := svc.Fetch(ctx, testMedia.ID())

			// Verify results
			if (err != nil) != tt.wantErr {
				t.Errorf("Fetch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if media.ID() != testMedia.ID() {
					t.Errorf("Fetch() got ID = %v, want %v", media.ID(), testMedia.ID())
				}
				if string(media.Bytes()) != string(testMedia.Bytes()) {
					t.Errorf("Fetch() got data = %v, want %v", string(media.Bytes()), string(testMedia.Bytes()))
				}
			}
		})
	}
}

func TestBlobMediaService_Delete(t *testing.T) {
	t.Parallel()

	svc, dataRepo, metaRepo, backrefRepo := setupMediaService(t)

	// Store test media
	testMedia := domain.NewMedia([]byte("test data"), domain.MediaMeta{
		Filename: "test.txt",
		Owner:    "testuser",
	})
	ctx := context_.WithUsername(context.Background(), "testuser")
	if err := svc.Store(ctx, testMedia); err != nil {
		t.Fatalf("failed to store test media: %v", err)
	}

	tests := []struct {
		name       string
		mediaID    domain.MediaID
		username   string
		dataErr    error
		metaErr    error
		backrefErr error
		wantErr    bool
	}{
		{
			name:     "deletes existing media",
			mediaID:  testMedia.ID(),
			username: "testuser",
			wantErr:  false,
		},
		{
			name:     "unauthorized user",
			mediaID:  testMedia.ID(),
			username: "otheruser",
			wantErr:  true,
		},
		{
			name:     "non-existent media",
			mediaID:  "nonexistent",
			username: "testuser",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dataRepo.deleteErr = tt.dataErr
			metaRepo.deleteErr = tt.metaErr
			backrefRepo.deleteErr = tt.backrefErr

			ctx := context_.WithUsername(context.Background(), tt.username)
			pruned, dataID, err := svc.Delete(ctx, tt.mediaID)

			if (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if pruned {
					if dataRepo.Exists(ctx, dataID) {
						t.Error("data blob was not deleted")
					}
				}
				if metaRepo.Exists(ctx, tt.mediaID) {
					t.Error("meta blob was not deleted")
				}
			}
		})
	}
}
