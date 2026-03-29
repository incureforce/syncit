package clientdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// ClientDB is the local SQLite database.
type ClientDB struct {
	sql *sql.DB
}

// Open opens an existing client database.
func Open(dbPath string) (*ClientDB, error) {
	d, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(1)
	if _, err := d.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = d.Close()
		return nil, err
	}
	// Allow concurrent CLI (e.g. sync push) while sync daemon holds the data-dir lock.
	if _, err := d.Exec(`PRAGMA busy_timeout = 10000`); err != nil {
		_ = d.Close()
		return nil, err
	}
	if err := migrate(d); err != nil {
		_ = d.Close()
		return nil, err
	}
	return &ClientDB{sql: d}, nil
}

// Close releases the handle.
func (c *ClientDB) Close() error {
	return c.sql.Close()
}

// DB exposes the pool for advanced use.
func (c *ClientDB) DB() *sql.DB {
	return c.sql
}

// InitNew creates directories, opens DB, inserts client row and server URL metadata
func InitNew(ctx context.Context, dbPath string, serverURL string) (*ClientDB, error) {
	d, err := Open(dbPath)
	if err != nil {
		return nil, err
	}

	var count int
	if err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM client`).Scan(&count); err != nil {
		_ = d.Close()
		return nil, err
	}
	if count > 0 {
		_ = d.Close()
		return nil, fmt.Errorf("client already initialized")
	}

	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		_ = d.Close()
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	cid := uuid.NewString()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO client (id, name, tags, created_at, deleted_at, updated_at)
		VALUES (?, '', '[]', ?, NULL, ?)
	`, cid, now, now); err != nil {
		_ = d.Close()
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO metadata (name, data) VALUES ('server_url', ?)
		ON CONFLICT(name) DO UPDATE SET data = excluded.data
	`, serverURL); err != nil {
		_ = d.Close()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		_ = d.Close()
		return nil, err
	}
	return d, nil
}

func (c *ClientDB) setMetadata(ctx context.Context, name, data string) error {
	_, err := c.sql.ExecContext(ctx, `
		INSERT INTO metadata (name, data) VALUES (?, ?)
		ON CONFLICT(name) DO UPDATE SET data = excluded.data
	`, name, data)
	return err
}

// ServerURL returns the configured server base URL.
func (c *ClientDB) ServerURL(ctx context.Context) (string, error) {
	var data string
	err := c.sql.QueryRowContext(ctx, `SELECT data FROM metadata WHERE name = 'server_url'`).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotConfigured
	}
	return data, err
}

// ErrNotConfigured means init has not run.
var ErrNotConfigured = errors.New("syncit not configured; run syncit init")

// ClientID returns this client's UUID.
func (c *ClientDB) ClientID(ctx context.Context) (string, error) {
	var id string
	err := c.sql.QueryRowContext(ctx, `SELECT id FROM client WHERE deleted_at IS NULL LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotConfigured
	}
	return id, err
}

// SetClientName updates display name.
func (c *ClientDB) SetClientName(ctx context.Context, name string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := c.sql.ExecContext(ctx, `UPDATE client SET name = ?, updated_at = ? WHERE deleted_at IS NULL`, name, now)
	return err
}

// Tags returns this client's tag set stored locally.
// The server copy drives who receives tagged files; we mirror tags here so
// tag ls / info work offline and tag add/del can update local state before PUT /v1/client/tags.
func (c *ClientDB) Tags(ctx context.Context) ([]string, error) {
	var raw string
	err := c.sql.QueryRowContext(ctx, `SELECT tags FROM client WHERE deleted_at IS NULL`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, err
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil, err
	}
	return tags, nil
}

// SetTags replaces client tags.
func (c *ClientDB) SetTags(ctx context.Context, tags []string) error {
	b, err := json.Marshal(tags)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = c.sql.ExecContext(ctx, `UPDATE client SET tags = ?, updated_at = ? WHERE deleted_at IS NULL`, string(b), now)
	return err
}

// AddTags merges unique tags.
func (c *ClientDB) AddTags(ctx context.Context, add []string) error {
	cur, err := c.Tags(ctx)
	if err != nil {
		return err
	}
	set := map[string]struct{}{}
	for _, t := range cur {
		set[t] = struct{}{}
	}
	for _, t := range add {
		if t != "" {
			set[t] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	return c.SetTags(ctx, out)
}

// RemoveTags removes tags (ignored if missing).
func (c *ClientDB) RemoveTags(ctx context.Context, del []string) error {
	cur, err := c.Tags(ctx)
	if err != nil {
		return err
	}
	rm := map[string]struct{}{}
	for _, t := range del {
		rm[t] = struct{}{}
	}
	out := make([]string, 0, len(cur))
	for _, t := range cur {
		if _, ok := rm[t]; !ok {
			out = append(out, t)
		}
	}
	return c.SetTags(ctx, out)
}

// MountRow is a mount record.
type MountRow struct {
	ID       string
	Name     string
	RootPath string
}

// ListMounts returns active mounts.
func (c *ClientDB) ListMounts(ctx context.Context) ([]MountRow, error) {
	rows, err := c.sql.QueryContext(ctx, `
		SELECT id, name, root_path FROM mount WHERE deleted_at IS NULL ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MountRow
	for rows.Next() {
		var m MountRow
		if err := rows.Scan(&m.ID, &m.Name, &m.RootPath); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AddMount inserts a mount.
func (c *ClientDB) AddMount(ctx context.Context, name, rootPath string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id := uuid.NewString()
	_, err := c.sql.ExecContext(ctx, `
		INSERT INTO mount (id, name, root_path, created_at, deleted_at, updated_at)
		VALUES (?, ?, ?, ?, NULL, ?)
	`, id, name, rootPath, now, now)
	return err
}

// DeleteMount soft-deletes a mount by name.
func (c *ClientDB) DeleteMount(ctx context.Context, name string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := c.sql.ExecContext(ctx, `
		UPDATE mount SET deleted_at = ?, updated_at = ? WHERE name = ? AND deleted_at IS NULL
	`, now, now, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ErrNotFound is returned when a named row is missing.
var ErrNotFound = errors.New("not found")

// MountByName returns a mount by name.
func (c *ClientDB) MountByName(ctx context.Context, name string) (MountRow, error) {
	var m MountRow
	err := c.sql.QueryRowContext(ctx, `
		SELECT id, name, root_path FROM mount WHERE name = ? AND deleted_at IS NULL
	`, name).Scan(&m.ID, &m.Name, &m.RootPath)
	if errors.Is(err, sql.ErrNoRows) {
		return m, ErrNotFound
	}
	return m, err
}
