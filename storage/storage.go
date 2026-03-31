package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store handles persistent storage for user bindings using SQLite.
type Store struct {
	db *sql.DB
}

// New creates a new Store instance with the database at the given path.
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return s, nil
}

func (s *Store) migrate() error {
	schema := `
		CREATE TABLE IF NOT EXISTS user_binding (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			user_id INTEGER NOT NULL,
			device_id TEXT NOT NULL,
			bound_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`

	_, err := s.db.Exec(schema)
	return err
}

// SaveUserBinding stores a user-device binding in the database.
func (s *Store) SaveUserBinding(ctx context.Context, userID int, deviceID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO user_binding (id, user_id, device_id, bound_at) VALUES (1, ?, ?, ?)`,
		userID, deviceID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to save user binding: %w", err)
	}
	return nil
}

// GetUserBinding retrieves the stored user-device binding from the database.
// Returns zero values if no binding exists.
func (s *Store) GetUserBinding(ctx context.Context) (userID int, deviceID string, err error) {
	err = s.db.QueryRowContext(ctx, "SELECT user_id, device_id FROM user_binding WHERE id = 1").Scan(&userID, &deviceID)
	if err == sql.ErrNoRows {
		return 0, "", nil
	}
	if err != nil {
		return 0, "", fmt.Errorf("failed to get user binding: %w", err)
	}
	return userID, deviceID, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
