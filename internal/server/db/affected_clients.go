package serverdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"go-syncit/internal/tags"
)

// ClientIDsAffectedByFilePush returns registered client IDs that should receive a notification for a new
// file version on the given global mount: linked to the mount, can see the file by tag rules, and not excludeClientID.
func (s *ServerDB) ClientIDsAffectedByFilePush(ctx context.Context, mountName string, fileTags []string, excludeClientID string) ([]string, error) {
	var mountID string
	err := s.sql.QueryRowContext(ctx, `SELECT id FROM mounts WHERE name = ? AND deleted_at IS NULL`, mountName).Scan(&mountID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	rows, err := s.sql.QueryContext(ctx, `
		SELECT c.id, c.tags
		FROM clients c
		INNER JOIN client_mounts cm ON cm.client_id = c.id AND cm.deleted_at IS NULL
		WHERE cm.mount_id = ? AND c.deleted_at IS NULL AND c.id != ?
	`, mountID, excludeClientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id, tagsRaw string
		if err := rows.Scan(&id, &tagsRaw); err != nil {
			return nil, err
		}
		var clientTags []string
		if err := json.Unmarshal([]byte(tagsRaw), &clientTags); err != nil {
			return nil, err
		}
		if len(fileTags) > 0 && !tags.Subset(fileTags, clientTags) {
			continue
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
