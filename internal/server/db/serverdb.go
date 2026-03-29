package serverdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ServerDB wraps the server-side SQLite database.
type ServerDB struct {
	sql *sql.DB
}

// OpenServer opens (or creates) the server database at dbPath.
func OpenServer(dbPath string) (*ServerDB, error) {
	d, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(1)
	if _, err := d.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = d.Close()
		return nil, err
	}
	if err := migrate(d); err != nil {
		_ = d.Close()
		return nil, err
	}
	if err := SeedDefaultSettings(context.Background(), d); err != nil {
		_ = d.Close()
		return nil, err
	}
	return &ServerDB{sql: d}, nil
}

// Close releases the database handle.
func (s *ServerDB) Close() error {
	return s.sql.Close()
}

// DB exposes the raw pool for advanced queries (handlers).
func (s *ServerDB) DB() *sql.DB {
	return s.sql
}

// RegisterClient inserts or updates a client row (registration / heartbeat).
func (s *ServerDB) RegisterClient(ctx context.Context, id, name string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.sql.ExecContext(ctx, `
		INSERT INTO clients (id, name, tags, created_at, deleted_at, updated_at)
		VALUES (?, ?, '[]', ?, NULL, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			updated_at = excluded.updated_at,
			deleted_at = NULL
	`, id, name, now, now)
	return err
}

// SetClientTags replaces the client's tag set.
func (s *ServerDB) SetClientTags(ctx context.Context, id string, tags []string) error {
	b, err := json.Marshal(tags)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.sql.ExecContext(ctx, `
		UPDATE clients SET tags = ?, updated_at = ?, synced_at = NULL WHERE id = ? AND deleted_at IS NULL
	`, string(b), now, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ErrNotFound is returned when a referenced row does not exist.
var ErrNotFound = errors.New("not found")

// GetClientTags returns tags for a client (for tests / future use).
func (s *ServerDB) GetClientTags(ctx context.Context, id string) ([]string, error) {
	var raw string
	err := s.sql.QueryRowContext(ctx, `SELECT tags FROM clients WHERE id = ? AND deleted_at IS NULL`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil, fmt.Errorf("tags json: %w", err)
	}
	return tags, nil
}
