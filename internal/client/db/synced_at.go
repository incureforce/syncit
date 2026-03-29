package clientdb

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// GetSyncedAt returns the last successful sync time (UTC), or false if unset or cleared.
func (c *ClientDB) GetSyncedAt(ctx context.Context) (t time.Time, ok bool, err error) {
	var raw sql.NullString
	err = c.sql.QueryRowContext(ctx, `SELECT synced_at FROM client WHERE deleted_at IS NULL LIMIT 1`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, ErrNotConfigured
	}
	if err != nil {
		return time.Time{}, false, err
	}
	if !raw.Valid || raw.String == "" {
		return time.Time{}, false, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw.String)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, raw.String)
		if err != nil {
			return time.Time{}, false, err
		}
	}
	return parsed.UTC(), true, nil
}

// SetSyncedAt records a successful sync completion time (UTC).
func (c *ClientDB) SetSyncedAt(ctx context.Context, at time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	s := at.UTC().Format(time.RFC3339Nano)
	_, err := c.sql.ExecContext(ctx, `UPDATE client SET synced_at = ?, updated_at = ? WHERE deleted_at IS NULL`, s, now)
	return err
}

// ClearSyncedAt clears the local sync cursor (e.g. after tag/mount subscription changes).
func (c *ClientDB) ClearSyncedAt(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := c.sql.ExecContext(ctx, `UPDATE client SET synced_at = NULL, updated_at = ? WHERE deleted_at IS NULL`, now)
	return err
}
