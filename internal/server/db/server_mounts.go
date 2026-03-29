package serverdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ClientMountRow is one server-side mount linked to the client.
type ClientMountRow struct {
	ID   string
	Name string
}

// ListGlobalMounts returns all active global mount names (not filtered by client).
func (s *ServerDB) ListGlobalMounts(ctx context.Context) ([]ClientMountRow, error) {
	rows, err := s.sql.QueryContext(ctx, `
		SELECT id, name FROM mounts WHERE deleted_at IS NULL ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ClientMountRow
	for rows.Next() {
		var r ClientMountRow
		if err := rows.Scan(&r.ID, &r.Name); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListMountsByClient returns active global mounts linked to this client.
func (s *ServerDB) ListMountsByClient(ctx context.Context, clientID string) ([]ClientMountRow, error) {
	rows, err := s.sql.QueryContext(ctx, `
		SELECT m.id, m.name
		FROM mounts m
		INNER JOIN client_mounts cm ON cm.mount_id = m.id AND cm.deleted_at IS NULL
		WHERE cm.client_id = ? AND m.deleted_at IS NULL
		ORDER BY m.name
	`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ClientMountRow
	for rows.Next() {
		var r ClientMountRow
		if err := rows.Scan(&r.ID, &r.Name); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func ensureMountTx(ctx context.Context, tx *sql.Tx, name string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM mounts WHERE name = ? AND deleted_at IS NULL`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id = uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mounts (id, name, created_at, deleted_at, updated_at)
		VALUES (?, ?, ?, NULL, ?)
	`, id, name, now, now)
	return id, err
}

func ensureClientMountTx(ctx context.Context, tx *sql.Tx, clientID, mountID string) error {
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO client_mounts (id, client_id, mount_id, created_at, deleted_at, updated_at)
		VALUES (?, ?, ?, ?, NULL, ?)
		ON CONFLICT(client_id, mount_id) DO UPDATE SET
			deleted_at = NULL,
			updated_at = excluded.updated_at
	`, id, clientID, mountID, now, now)
	return err
}

// SyncClientMounts replaces server-side links for this client with the given mount names
// (soft-deletes links not listed, then ensures each name exists and is linked).
func (s *ServerDB) SyncClientMounts(ctx context.Context, clientID string, mountNames []string) error {
	if _, err := s.GetClientTags(ctx, clientID); err != nil {
		return err
	}
	seen := make(map[string]struct{})
	var wanted []string
	for _, raw := range mountNames {
		n := strings.TrimSpace(raw)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		wanted = append(wanted, n)
	}

	tx, err := s.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if len(wanted) == 0 {
		_, err = tx.ExecContext(ctx, `
			UPDATE client_mounts SET deleted_at = ?, updated_at = ?
			WHERE client_id = ? AND deleted_at IS NULL
		`, now, now, clientID)
	} else {
		ph := strings.Repeat("?,", len(wanted))
		ph = ph[:len(ph)-1]
		query := fmt.Sprintf(`
			UPDATE client_mounts SET deleted_at = ?, updated_at = ?
			WHERE client_id = ? AND deleted_at IS NULL
			AND mount_id NOT IN (
				SELECT id FROM mounts WHERE name IN (%s) AND deleted_at IS NULL
			)`, ph)
		args := []any{now, now, clientID}
		for _, n := range wanted {
			args = append(args, n)
		}
		_, err = tx.ExecContext(ctx, query, args...)
	}
	if err != nil {
		return err
	}

	for _, name := range wanted {
		mid, err := ensureMountTx(ctx, tx, name)
		if err != nil {
			return err
		}
		if err := ensureClientMountTx(ctx, tx, clientID, mid); err != nil {
			return err
		}
	}
	now2 := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `
		UPDATE clients SET synced_at = NULL, updated_at = ? WHERE id = ? AND deleted_at IS NULL
	`, now2, clientID); err != nil {
		return err
	}
	return tx.Commit()
}
