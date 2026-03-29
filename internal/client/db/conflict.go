package clientdb

import (
	"context"
	"encoding/json"
	"time"
)

// SetConflict marks a tracked file as conflicted or clears the flag.
func (c *ClientDB) SetConflict(ctx context.Context, id string, conflict bool) error {
	v := 0
	if conflict {
		v = 1
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := c.sql.ExecContext(ctx, `
		UPDATE mount_file SET conflict = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL
	`, v, now, id)
	return err
}

// ApplyPull updates local state after taking remote content and tag metadata.
func (c *ClientDB) ApplyPull(ctx context.Context, id string, contentHash string, remoteVersion int, remoteTags []string) error {
	if remoteTags == nil {
		remoteTags = []string{}
	}
	tagsJSON, err := json.Marshal(remoteTags)
	if err != nil {
		return err
	}
	s := string(tagsJSON)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = c.sql.ExecContext(ctx, `
		UPDATE mount_file SET
			local_file_hash = ?,
			remote_file_hash = ?,
			remote_file_version = ?,
			local_file_version = ?,
			local_file_tags = ?,
			remote_file_tags = ?,
			conflict = 0,
			updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`, contentHash, contentHash, remoteVersion, remoteVersion, s, s, now, id)
	return err
}
