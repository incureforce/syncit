package serverdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"go-syncit/internal/tags"
)

// LatestFileInfo is the newest server version for a path (tag visibility applied).
type LatestFileInfo struct {
	Version   int    `json:"version"`
	FileHash  string `json:"file_hash"`
	FileSize  int64  `json:"file_size"`
	BlobKey   string `json:"blob_key"`
	FileTags  []string
	MountName string
	Path      string
}

// LatestVisibleFile returns the latest version if the client may see this file.
func (s *ServerDB) LatestVisibleFile(ctx context.Context, clientID, mountName, relPath string) (*LatestFileInfo, error) {
	clientTags, err := s.GetClientTags(ctx, clientID)
	if err != nil {
		return nil, err
	}
	var mountID string
	err = s.sql.QueryRowContext(ctx, `
		SELECT m.id FROM mounts m
		INNER JOIN client_mounts cm ON cm.mount_id = m.id AND cm.client_id = ? AND cm.deleted_at IS NULL
		WHERE m.name = ? AND m.deleted_at IS NULL
	`, clientID, mountName).Scan(&mountID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var mfID, tagsRaw string
	err = s.sql.QueryRowContext(ctx, `
		SELECT id, tags FROM mount_files WHERE mount_id = ? AND path = ? AND deleted_at IS NULL
	`, mountID, relPath).Scan(&mfID, &tagsRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var fileTags []string
	if err := json.Unmarshal([]byte(tagsRaw), &fileTags); err != nil {
		return nil, err
	}
	if len(fileTags) > 0 && !tags.Subset(fileTags, clientTags) {
		return nil, ErrNotFound
	}

	var info LatestFileInfo
	err = s.sql.QueryRowContext(ctx, `
		SELECT v.file_version, v.file_hash, v.file_size, v.blob_key
		FROM mount_file_versions v
		WHERE v.mount_file_id = ? AND v.deleted_at IS NULL
		ORDER BY v.file_version DESC LIMIT 1
	`, mfID).Scan(&info.Version, &info.FileHash, &info.FileSize, &info.BlobKey)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	info.FileTags = fileTags
	info.MountName = mountName
	info.Path = relPath
	return &info, nil
}

// LatestVisibleFiles returns the latest version of every mount_file for mounts this client has linked, filtered by the same tag rules as LatestVisibleFile.
// mountName empty means all such mounts.
func (s *ServerDB) LatestVisibleFiles(ctx context.Context, clientID, mountName string) ([]LatestFileInfo, error) {
	clientTags, err := s.GetClientTags(ctx, clientID)
	if err != nil {
		return nil, err
	}

	args := []any{clientID}
	mountFilter := ""
	if mountName != "" {
		mountFilter = " AND m.name = ?"
		args = append(args, mountName)
	}

	rows, err := s.sql.QueryContext(ctx, `
		SELECT m.name, mf.path, mf.tags, v.file_version, v.file_hash, v.file_size, v.blob_key
		FROM client_mounts cm
		INNER JOIN mounts m ON m.id = cm.mount_id AND m.deleted_at IS NULL
		INNER JOIN mount_files mf ON mf.mount_id = m.id AND mf.deleted_at IS NULL
		INNER JOIN mount_file_versions v ON v.mount_file_id = mf.id AND v.deleted_at IS NULL
		INNER JOIN (
			SELECT mount_file_id, MAX(file_version) AS max_v
			FROM mount_file_versions
			WHERE deleted_at IS NULL
			GROUP BY mount_file_id
		) latest ON latest.mount_file_id = mf.id AND v.file_version = latest.max_v
		WHERE cm.client_id = ? AND cm.deleted_at IS NULL`+mountFilter+`
		ORDER BY m.name, mf.path
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LatestFileInfo
	for rows.Next() {
		var info LatestFileInfo
		var tagsRaw string
		if err := rows.Scan(&info.MountName, &info.Path, &tagsRaw, &info.Version, &info.FileHash, &info.FileSize, &info.BlobKey); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(tagsRaw), &info.FileTags); err != nil {
			return nil, err
		}
		if len(info.FileTags) > 0 && !tags.Subset(info.FileTags, clientTags) {
			continue
		}
		out = append(out, info)
	}
	return out, rows.Err()
}
