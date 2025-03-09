package authsvc_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mkrupp/homecase-michael/internal/domain"
	"github.com/mkrupp/homecase-michael/internal/infra/logging"
	"github.com/mkrupp/homecase-michael/internal/svc/authsvc"
)

// mockUserRepository implements user.Repository for testing.
type mockUserRepository struct {
	users map[string]*domain.User
	err   error
	m     sync.Mutex
}

func (m *mockUserRepository) CreateUser(_ context.Context, username string, passwordHash []byte) error {
	m.m.Lock()
	defer m.m.Unlock()

	if m.err != nil {
		return m.err
	}
	if _, exists := m.users[username]; exists {
		return domain.ErrUserAlreadyExists
	}
	m.users[username] = &domain.User{
		ID:           int64(len(m.users) + 1),
		Username:     username,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now().Unix(),
	}
	return nil
}

func (m *mockUserRepository) GetUserByUsername(_ context.Context, username string) (*domain.User, bool, error) {
	if m.err != nil {
		return nil, false, m.err
	}
	user, exists := m.users[username]
	if !exists {
		return nil, false, domain.ErrUserNotFound
	}
	return user, true, nil
}

func (m *mockUserRepository) Close() error {
	return m.err
}

func newMockUserRepo() *mockUserRepository {
	return &mockUserRepository{
		users: make(map[string]*domain.User),
	}
}

var ErrRepoError = errors.New("repository error")

func setupTestService(t *testing.T) (*authsvc.AuthService, *mockUserRepository) {
	t.Helper()

	// Generate temporary signing key
	signingKey, err := authsvc.GeneratePrivateKey(2048)
	if err != nil {
		t.Fatalf("failed to generate signing key: %v", err)
	}

	mockRepo := newMockUserRepo()
	cfg := authsvc.AuthConfig{
		TokenDuration: 3600,
	}

	svc := &authsvc.AuthService{
		Config:     cfg,
		UserRepo:   mockRepo,
		Log:        logging.GetLogger("test.authsvc"),
		SigningKey: signingKey,
	}

	return svc, mockRepo
}

//nolint:paralleltest
func TestAuthService_RegisterUser(t *testing.T) {
	svc, mockRepo := setupTestService(t)

	tests := []struct {
		name     string
		username string
		password string
		repoErr  error
		wantErr  error
	}{
		{
			name:     "successful registration",
			username: "newuser",
			password: "password123",
			wantErr:  nil,
		},
		{
			name:     "duplicate username",
			username: "existinguser",
			password: "password123",
			wantErr:  domain.ErrUserAlreadyExists,
		},
		{
			name:     "repository error",
			username: "erroruser",
			password: "password123",
			repoErr:  ErrRepoError,
			wantErr:  ErrRepoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test case
			if tt.name == "duplicate username" {
				_ = svc.RegisterUser(context.Background(), tt.username, "oldpass")
			}
			mockRepo.err = tt.repoErr

			// Execute test
			err := svc.RegisterUser(context.Background(), tt.username, tt.password)

			// Verify results
			if (err != nil) != (tt.wantErr != nil) {
				t.Errorf("RegisterUser() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("RegisterUser() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAuthService_Login(t *testing.T) {
	t.Parallel()

	svc, mockRepo := setupTestService(t)

	// Create test user
	testPassword := "testpass123"
	hasher := sha256.New()
	hasher.Write([]byte(testPassword))
	passwordHash := hasher.Sum(nil)

	mockRepo.users["testuser"] = &domain.User{
		ID:           1,
		Username:     "testuser",
		PasswordHash: passwordHash,
		CreatedAt:    time.Now().Unix(),
	}

	tests := []struct {
		name     string
		username string
		password string
		repoErr  error
		wantErr  error
	}{
		{
			name:     "successful login",
			username: "testuser",
			password: "testpass123",
			wantErr:  nil,
		},
		{
			name:     "wrong password",
			username: "testuser",
			password: "wrongpass",
			wantErr:  domain.ErrInvalidCredentials,
		},
		{
			name:     "user not found",
			username: "nonexistent",
			password: "anypass",
			wantErr:  domain.ErrInvalidCredentials,
		},
		{
			name:     "repository error",
			username: "testuser",
			password: "testpass123",
			repoErr:  ErrRepoError,
			wantErr:  ErrRepoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockRepo.err = tt.repoErr

			// Execute test
			token, err := svc.Login(context.Background(), tt.username, tt.password)

			// Verify results
			if (err != nil) != (tt.wantErr != nil) {
				t.Errorf("Login() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("Login() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				// Verify token can be validated
				_, err = svc.ValidateToken(context.Background(), token)
				if err != nil {
					t.Errorf("Login() generated invalid token: %v", err)
				}
			}
		})
	}
}

func TestAuthService_ValidateToken(t *testing.T) {
	t.Parallel()

	svc, _ := setupTestService(t)

	// Generate a valid token
	ctx := context.Background()
	svc.RegisterUser(ctx, "testuser", "testpass")
	validToken, err := svc.Login(ctx, "testuser", "testpass")
	if err != nil {
		t.Fatalf("failed to generate test token: %v", err)
	}

	tests := []struct {
		name        string
		token       string
		wantErr     error
		wantExpired bool
	}{
		{
			name:    "valid token",
			token:   validToken,
			wantErr: nil,
		},
		{
			name:    "invalid token format",
			token:   "invalid-token",
			wantErr: domain.ErrInvalidAuthToken,
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: domain.ErrInvalidAuthToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			token, err := svc.ValidateToken(ctx, tt.token)

			if (err != nil) != (tt.wantErr != nil) {
				t.Errorf("ValidateToken() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateToken() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				if token.Username != "testuser" {
					t.Errorf("ValidateToken() username = %v, want %v", token.Username, "testuser")
				}
				if token.ExpiresAt <= time.Now().Unix() {
					t.Error("ValidateToken() token already expired")
				}
			}
		})
	}
}
