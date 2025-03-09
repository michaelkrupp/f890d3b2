package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/mkrupp/homecase-michael/internal/domain"
	"github.com/mkrupp/homecase-michael/internal/infra/logging"
)

// SQLiteUserRepositoryConfig holds configuration for the SQLite user repository.
type SQLiteUserRepositoryConfig struct {
	// DatabasePath is the filesystem path to the SQLite database file
	DatabasePath string `env:"DATABASE_PATH" default:"var/storage/authsvc.db"`
}

// SQLiteUserRepository implements Repository using SQLite as the storage backend.
type SQLiteUserRepository struct {
	db        *sql.DB
	log       logging.Logger
	writeLock *sync.Mutex // go-sqlite does not support concurrent writes
}

var _ Repository = (*SQLiteUserRepository)(nil)

// SQLiteUserRepositoryFactory creates a factory function that returns a new SQLiteUserRepository.
// The factory function implements the RepositoryFactory type.
func SQLiteUserRepositoryFactory(cfg SQLiteUserRepositoryConfig) RepositoryFactory {
	return func() (Repository, error) {
		return NewSQLiteUserRepository(cfg)
	}
}

// NewSQLiteUserRepository creates a new SQLiteUserRepository with the given configuration.
// It initializes the database connection and creates the schema if needed.
// Returns an error if database connection or initialization fails.
func NewSQLiteUserRepository(cfg SQLiteUserRepositoryConfig) (*SQLiteUserRepository, error) {
	log := logging.GetLogger("repo.user.sqlite_user_repository").With(
		logging.Group("db", "path", cfg.DatabasePath),
	)

	db, err := sql.Open("sqlite", cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if err := initializeDB(db); err != nil {
		return nil, fmt.Errorf("initialize db: %w", err)
	}

	db.SetConnMaxLifetime(5 * time.Minute)

	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	return &SQLiteUserRepository{
		db:        db,
		log:       log,
		writeLock: new(sync.Mutex),
	}, nil
}

func initializeDB(db *sql.DB) (err error) {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT    UNIQUE NOT NULL,
			password_hash BLOB    NOT NULL,
			created_at    INTEGER NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	return nil
}

// CreateUser implements Repository.CreateUser using SQLite.
func (r *SQLiteUserRepository) CreateUser(ctx context.Context, username string, passwordHash []byte) (err error) {
	r.writeLock.Lock()
	defer r.writeLock.Unlock()

	_, err = r.db.Exec(
		"INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)",
		username,
		passwordHash,
		time.Now().Unix(),
	)
	if err != nil {
		var liteErr *sqlite.Error
		if errors.As(err, &liteErr) {
			switch liteErr.Code() {
			case sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
				fallthrough
			case sqlite3.SQLITE_CONSTRAINT_UNIQUE:
				err = errors.Join(domain.ErrUserAlreadyExists, err)
			default:
				break
			}
		}

		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}

// GetUserByUsername implements Repository.GetUserByUsername using SQLite.
func (r *SQLiteUserRepository) GetUserByUsername(ctx context.Context, username string) (*domain.User, bool, error) {
	var user domain.User

	err := r.db.QueryRow(
		"SELECT id, username, password_hash, created_at FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = errors.Join(domain.ErrUserNotFound, err)
		}

		return nil, false, fmt.Errorf("query user: %w", err)
	}

	return &user, true, nil
}

// Close implements Repository.Close by closing the database connection.
func (r *SQLiteUserRepository) Close() error {
	if err := r.db.Close(); err != nil {
		return fmt.Errorf("close db: %w", err)
	}

	return nil
}
