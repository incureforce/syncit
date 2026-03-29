package serverdb

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// GetClientSyncedAt returns the client's last reported sync cursor (UTC RFC3339), or false if unset.
func (s *ServerDB) GetClientSyncedAt(ctx context.Context, clientID string) (t time.Time, ok bool, err error) {
	var raw sql.NullString
	err = s.sql.QueryRowContext(ctx, `SELECT synced_at FROM clients WHERE id = ? AND deleted_at IS NULL`, clientID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, ErrNotFound
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

// SetClientSyncedAt sets or clears the server-side sync cursor for this client.
func (s *ServerDB) SetClientSyncedAt(ctx context.Context, clientID string, at *time.Time) error {
	var val any
	if at != nil {
		val = at.UTC().Format(time.RFC3339Nano)
	} else {
		val = nil
	}
	res, err := s.sql.ExecContext(ctx, `
		UPDATE clients SET synced_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL
	`, val, time.Now().UTC().Format(time.RFC3339Nano), clientID)
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
