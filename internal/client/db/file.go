package clientdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"go-syncit/internal/tags"
)

// TrackedFile is one row in mount_file.
type TrackedFile struct {
	ID            string
	MountID       string
	Path          string
	Tags          []string
	RemoteTags    []string // last known server tags after push/pull
	Version       int
	RemoteVersion int
	BlobKey       string
	FileSize      int64
	FileHash      string
	RemoteHash    string
	Conflict      bool
	MountName     string
}

// AddTrackedFile inserts or updates a tracked file (local version bump).
func (c *ClientDB) AddTrackedFile(ctx context.Context, mountID, relPath string, fileTags []string, fileHash string, fileSize int64) error {
	tagsJSON, err := json.Marshal(fileTags)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	var existingID string
	err = c.sql.QueryRowContext(ctx, `
		SELECT id FROM mount_file WHERE mount_id = ? AND local_file_path = ? AND deleted_at IS NULL
	`, mountID, relPath).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		id := uuid.NewString()
		_, err = c.sql.ExecContext(ctx, `
			INSERT INTO mount_file (
				id, mount_id, local_file_path, local_file_tags, remote_file_tags, local_file_version, remote_file_version, blob_key, file_size, local_file_hash, remote_file_hash, conflict,
				created_at, deleted_at, updated_at
			) VALUES (?, ?, ?, ?, '[]', 1, 0, '', ?, ?, '', 0, ?, NULL, ?)
		`, id, mountID, relPath, string(tagsJSON), fileSize, fileHash, now, now)
		return err
	}
	if err != nil {
		return err
	}
	_, err = c.sql.ExecContext(ctx, `
		UPDATE mount_file SET
			local_file_tags = ?,
			local_file_version = local_file_version + 1,
			file_size = ?,
			local_file_hash = ?,
			conflict = 0,
			updated_at = ?
		WHERE id = ?
	`, string(tagsJSON), fileSize, fileHash, now, existingID)
	return err
}

// InsertNewFromRemotePull registers a path that exists only on the server after download (sync discover).
// If the path is already tracked, returns (false, nil) without changing rows.
func (c *ClientDB) InsertNewFromRemotePull(ctx context.Context, mountID, relPath string, fileTags []string, fileHash string, fileSize int64, remoteVersion int, blobKey string) (inserted bool, err error) {
	var n int
	err = c.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM mount_file WHERE mount_id = ? AND local_file_path = ? AND deleted_at IS NULL`, mountID, relPath).Scan(&n)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	tagsJSON, err := json.Marshal(fileTags)
	if err != nil {
		return false, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id := uuid.NewString()
	_, err = c.sql.ExecContext(ctx, `
		INSERT INTO mount_file (
			id, mount_id, local_file_path, local_file_tags, remote_file_tags, local_file_version, remote_file_version, blob_key, file_size, local_file_hash, remote_file_hash, conflict,
			created_at, deleted_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, NULL, ?)
	`, id, mountID, relPath, string(tagsJSON), string(tagsJSON), remoteVersion, remoteVersion, blobKey, fileSize, fileHash, fileHash, now, now)
	if err != nil {
		return false, err
	}
	return true, nil
}

// ListTrackedWithMount returns tracked files joined with mount name.
func (c *ClientDB) ListTrackedWithMount(ctx context.Context) ([]TrackedFile, error) {
	rows, err := c.sql.QueryContext(ctx, `
		SELECT mf.id, mf.mount_id, mf.local_file_path, mf.local_file_tags, mf.remote_file_tags, mf.local_file_version, mf.remote_file_version, mf.blob_key, mf.file_size, mf.local_file_hash, mf.remote_file_hash, mf.conflict, m.name
		FROM mount_file mf
		JOIN mount m ON m.id = mf.mount_id
		WHERE mf.deleted_at IS NULL AND m.deleted_at IS NULL
		ORDER BY m.name, mf.local_file_path
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrackedFile
	for rows.Next() {
		var tf TrackedFile
		var tagsRaw, remoteTagsRaw string
		var conflictInt int
		if err := rows.Scan(&tf.ID, &tf.MountID, &tf.Path, &tagsRaw, &remoteTagsRaw, &tf.Version, &tf.RemoteVersion, &tf.BlobKey, &tf.FileSize, &tf.FileHash, &tf.RemoteHash, &conflictInt, &tf.MountName); err != nil {
			return nil, err
		}
		tf.Conflict = conflictInt != 0
		if err := json.Unmarshal([]byte(tagsRaw), &tf.Tags); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(remoteTagsRaw), &tf.RemoteTags); err != nil {
			return nil, err
		}
		out = append(out, tf)
	}
	return out, rows.Err()
}

// MarkPushed updates remote state after a successful push.
func (c *ClientDB) MarkPushed(ctx context.Context, id string, remoteVersion int, blobKey string, pushedTags []string) error {
	if pushedTags == nil {
		pushedTags = []string{}
	}
	tagsJSON, err := json.Marshal(pushedTags)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = c.sql.ExecContext(ctx, `
		UPDATE mount_file SET
			remote_file_version = ?,
			remote_file_hash = local_file_hash,
			remote_file_tags = ?,
			blob_key = ?,
			updated_at = ?
		WHERE id = ?
	`, remoteVersion, string(tagsJSON), blobKey, now, id)
	return err
}

// ListNeedingPush returns tracked files whose local content or tags differ from the last pushed server state, excluding conflicts.
func (c *ClientDB) ListNeedingPush(ctx context.Context) ([]TrackedFile, error) {
	all, err := c.ListTrackedWithMount(ctx)
	if err != nil {
		return nil, err
	}
	var out []TrackedFile
	for _, tf := range all {
		if tf.Conflict {
			continue
		}
		if tf.RemoteHash == "" || tf.RemoteHash != tf.FileHash || !tags.Equal(tf.Tags, tf.RemoteTags) {
			out = append(out, tf)
		}
	}
	return out, nil
}

// MountRoot returns the absolute root path for a mount id.
func (c *ClientDB) MountRoot(ctx context.Context, mountID string) (string, error) {
	var root string
	err := c.sql.QueryRowContext(ctx, `SELECT root_path FROM mount WHERE id = ? AND deleted_at IS NULL`, mountID).Scan(&root)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return root, err
}

// TrackedByMountAndPath returns a tracked file by mount name and relative path.
func (c *ClientDB) TrackedByMountAndPath(ctx context.Context, mountName, relPath string) (TrackedFile, error) {
	var tf TrackedFile
	var tagsRaw string
	var conflictInt int
	var remoteTagsRaw string
	err := c.sql.QueryRowContext(ctx, `
		SELECT mf.id, mf.mount_id, mf.local_file_path, mf.local_file_tags, mf.remote_file_tags, mf.local_file_version, mf.remote_file_version, mf.blob_key, mf.file_size, mf.local_file_hash, mf.remote_file_hash, mf.conflict, m.name
		FROM mount_file mf
		JOIN mount m ON m.id = mf.mount_id
		WHERE m.name = ? AND mf.local_file_path = ? AND mf.deleted_at IS NULL AND m.deleted_at IS NULL
	`, mountName, relPath).Scan(&tf.ID, &tf.MountID, &tf.Path, &tagsRaw, &remoteTagsRaw, &tf.Version, &tf.RemoteVersion, &tf.BlobKey, &tf.FileSize, &tf.FileHash, &tf.RemoteHash, &conflictInt, &tf.MountName)
	if errors.Is(err, sql.ErrNoRows) {
		return tf, ErrNotFound
	}
	if err != nil {
		return tf, err
	}
	tf.Conflict = conflictInt != 0
	if err := json.Unmarshal([]byte(tagsRaw), &tf.Tags); err != nil {
		return tf, err
	}
	if err := json.Unmarshal([]byte(remoteTagsRaw), &tf.RemoteTags); err != nil {
		return tf, err
	}
	return tf, nil
}

// FileByID returns a tracked file by primary key.
func (c *ClientDB) FileByID(ctx context.Context, id string) (TrackedFile, error) {
	var tf TrackedFile
	var tagsRaw string
	var conflictInt int
	var remoteTagsRaw string
	err := c.sql.QueryRowContext(ctx, `
		SELECT mf.id, mf.mount_id, mf.local_file_path, mf.local_file_tags, mf.remote_file_tags, mf.local_file_version, mf.remote_file_version, mf.blob_key, mf.file_size, mf.local_file_hash, mf.remote_file_hash, mf.conflict, m.name
		FROM mount_file mf
		JOIN mount m ON m.id = mf.mount_id
		WHERE mf.id = ? AND mf.deleted_at IS NULL
	`, id).Scan(&tf.ID, &tf.MountID, &tf.Path, &tagsRaw, &remoteTagsRaw, &tf.Version, &tf.RemoteVersion, &tf.BlobKey, &tf.FileSize, &tf.FileHash, &tf.RemoteHash, &conflictInt, &tf.MountName)
	if errors.Is(err, sql.ErrNoRows) {
		return tf, ErrNotFound
	}
	if err != nil {
		return tf, err
	}
	tf.Conflict = conflictInt != 0
	if err := json.Unmarshal([]byte(tagsRaw), &tf.Tags); err != nil {
		return tf, err
	}
	if err := json.Unmarshal([]byte(remoteTagsRaw), &tf.RemoteTags); err != nil {
		return tf, err
	}
	return tf, nil
}
