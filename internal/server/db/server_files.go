package serverdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"go-syncit/internal/tags"

	"github.com/google/uuid"
)

// EnsureMount returns the server mount id for a global mount name, creating if needed.
func (s *ServerDB) EnsureMount(ctx context.Context, name string) (string, error) {
	var id string
	err := s.sql.QueryRowContext(ctx, `SELECT id FROM mounts WHERE name = ? AND deleted_at IS NULL`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id = uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.sql.ExecContext(ctx, `
		INSERT INTO mounts (id, name, created_at, deleted_at, updated_at)
		VALUES (?, ?, ?, NULL, ?)
	`, id, name, now, now)
	return id, err
}

// EnsureClientMount links a registered client to a global mount (idempotent).
func (s *ServerDB) EnsureClientMount(ctx context.Context, clientID, mountID string) error {
	var existingID string
	err := s.sql.QueryRowContext(ctx, `
		SELECT id FROM client_mounts
		WHERE client_id = ? AND mount_id = ? AND deleted_at IS NULL
	`, clientID, mountID).Scan(&existingID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	// Check for a soft-deleted row to reuse
	var deletedID string
	err = s.sql.QueryRowContext(ctx, `
		SELECT id FROM client_mounts
		WHERE client_id = ? AND mount_id = ? AND deleted_at IS NOT NULL
		LIMIT 1
	`, clientID, mountID).Scan(&deletedID)
	if err == nil {
		_, err = s.sql.ExecContext(ctx, `
			UPDATE client_mounts SET deleted_at = NULL, updated_at = ? WHERE id = ?
		`, now, deletedID)
		return err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	id := uuid.NewString()
	_, err = s.sql.ExecContext(ctx, `
		INSERT INTO client_mounts (id, client_id, mount_id, created_at, deleted_at, updated_at)
		VALUES (?, ?, ?, ?, NULL, ?)
	`, id, clientID, mountID, now, now)
	return err
}

// PushFileResult is returned after recording a new version.
type PushFileResult struct {
	MountFileID string
	Version     int
}

// DeleteFileResult describes a remote soft-delete request outcome.
type DeleteFileResult struct {
	Deleted  bool
	FileTags []string
}

// PushFileVersion validates tag policy, ensures mount/file rows, appends a version.
func (s *ServerDB) PushFileVersion(ctx context.Context, clientID, mountName, relPath string, fileTags []string, blobKey string, fileSize int64, fileHash string) (*PushFileResult, error) {
	clientTags, err := s.GetClientTags(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if len(fileTags) > 0 && !tags.Subset(fileTags, clientTags) {
		return nil, ErrTagPolicy
	}

	mountID, err := s.EnsureMount(ctx, mountName)
	if err != nil {
		return nil, err
	}
	if err := s.EnsureClientMount(ctx, clientID, mountID); err != nil {
		return nil, err
	}

	tagsJSON, err := json.Marshal(fileTags)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	tx, err := s.sql.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var mfID string
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM mount_files WHERE mount_id = ? AND path = ? AND deleted_at IS NULL
	`, mountID, relPath).Scan(&mfID)
	if errors.Is(err, sql.ErrNoRows) {
		mfID = uuid.NewString()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO mount_files (id, mount_id, path, tags, created_at, deleted_at, updated_at)
			VALUES (?, ?, ?, ?, ?, NULL, ?)
		`, mfID, mountID, relPath, string(tagsJSON), now, now)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE mount_files SET tags = ?, updated_at = ? WHERE id = ?
		`, string(tagsJSON), now, mfID)
		if err != nil {
			return nil, err
		}
	}

	var maxVer sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		SELECT MAX(file_version) FROM mount_file_versions WHERE mount_file_id = ? AND deleted_at IS NULL
	`, mfID).Scan(&maxVer)
	if err != nil {
		return nil, err
	}
	next := 1
	if maxVer.Valid {
		next = int(maxVer.Int64) + 1
	}

	vid := uuid.NewString()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mount_file_versions (id, mount_file_id, file_version, blob_key, file_size, file_hash, created_at, deleted_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?)
	`, vid, mfID, next, blobKey, fileSize, fileHash, now, now)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &PushFileResult{MountFileID: mfID, Version: next}, nil
}

// ErrTagPolicy means the client's tags do not satisfy the file's tag requirements.
var ErrTagPolicy = errors.New("tag policy violation")

// SoftDeleteFile marks a mount file deleted if it exists and the client is allowed by file tags.
// Returns Deleted=false when the file is already deleted or does not exist.
func (s *ServerDB) SoftDeleteFile(ctx context.Context, clientID, mountName, relPath string) (*DeleteFileResult, error) {
	clientTags, err := s.GetClientTags(ctx, clientID)
	if err != nil {
		return nil, err
	}

	var mountID string
	err = s.sql.QueryRowContext(ctx, `
		SELECT m.id
		FROM mounts m
		INNER JOIN client_mounts cm ON cm.mount_id = m.id AND cm.client_id = ? AND cm.deleted_at IS NULL
		WHERE m.name = ? AND m.deleted_at IS NULL
	`, clientID, mountName).Scan(&mountID)
	if errors.Is(err, sql.ErrNoRows) {
		return &DeleteFileResult{Deleted: false, FileTags: nil}, nil
	}
	if err != nil {
		return nil, err
	}

	var mfID, tagsRaw string
	var deletedAt sql.NullString
	err = s.sql.QueryRowContext(ctx, `
		SELECT id, tags, deleted_at
		FROM mount_files
		WHERE mount_id = ? AND path = ?
	`, mountID, relPath).Scan(&mfID, &tagsRaw, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return &DeleteFileResult{Deleted: false, FileTags: nil}, nil
	}
	if err != nil {
		return nil, err
	}

	var fileTags []string
	if err := json.Unmarshal([]byte(tagsRaw), &fileTags); err != nil {
		return nil, err
	}
	if len(fileTags) > 0 && !tags.Subset(fileTags, clientTags) {
		return nil, ErrTagPolicy
	}
	if deletedAt.Valid {
		return &DeleteFileResult{Deleted: false, FileTags: fileTags}, nil
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.sql.ExecContext(ctx, `
		UPDATE mount_files SET deleted_at = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`, now, now, mfID)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	return &DeleteFileResult{Deleted: n > 0, FileTags: fileTags}, nil
}
