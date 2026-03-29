package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	clientdb "go-syncit/internal/client/db"
	clienthttp "go-syncit/internal/client/http"
)

// RunPullFull lists the latest catalog per mount, discovers new paths, then pulls every tracked file.
// It does not consult synced_at (full repair/discover path).
func RunPullFull(ctx context.Context, db *clientdb.ClientDB, api *clienthttp.Client, cid string) error {
	mounts, err := db.ListMounts(ctx)
	if err != nil {
		return err
	}
	localMounts := make(map[string]clientdb.MountRow, len(mounts))
	for _, m := range mounts {
		localMounts[m.Name] = m
	}

	tracked, err := db.ListTrackedWithMount(ctx)
	if err != nil {
		return err
	}
	trackedKey := make(map[string]struct{}, len(tracked))
	for _, tf := range tracked {
		trackedKey[pullTrackKey(tf.MountName, tf.Path)] = struct{}{}
	}

	var catalog []clienthttp.CatalogFile
	for _, m := range mounts {
		files, err := api.ListFilesLatest(cid, m.Name)
		if err != nil {
			return fmt.Errorf("files/latest %q: %w", m.Name, err)
		}
		catalog = append(catalog, files...)
	}
	for _, e := range catalog {
		lm, ok := localMounts[e.MountName]
		if !ok {
			continue
		}
		k := pullTrackKey(e.MountName, e.Path)
		if _, exists := trackedKey[k]; exists {
			continue
		}
		if err := pullDiscoverNew(ctx, db, api, cid, lm, e); err != nil {
			return err
		}
		trackedKey[k] = struct{}{}
	}

	tracked, err = db.ListTrackedWithMount(ctx)
	if err != nil {
		return err
	}
	for _, tf := range tracked {
		if tf.Conflict {
			fmt.Printf("%s %s\n", syncBad("skip (conflict)"), listKey(tf.Path))
			continue
		}
		if err := pullOne(ctx, db, api, cid, tf); err != nil {
			return err
		}
	}
	return nil
}

// AdvanceSyncState updates local and server synced_at after a successful pull (full or incremental).
func AdvanceSyncState(ctx context.Context, db *clientdb.ClientDB, api *clienthttp.Client, cid string) error {
	now := time.Now().UTC()
	if err := db.SetSyncedAt(ctx, now); err != nil {
		return err
	}
	return api.PutSyncState(cid, &now)
}

// PullPathAfterEvent runs a narrow pull for one server path (discover or pullOne).
func PullPathAfterEvent(ctx context.Context, db *clientdb.ClientDB, api *clienthttp.Client, cid, mountName, relPath string) error {
	lm, err := db.MountByName(ctx, mountName)
	if err != nil {
		if errors.Is(err, clientdb.ErrNotFound) {
			return nil
		}
		return err
	}
	tf, err := db.TrackedByMountAndPath(ctx, mountName, relPath)
	if err != nil {
		if errors.Is(err, clientdb.ErrNotFound) {
			srv, err := api.GetFileLatest(cid, mountName, relPath)
			if err != nil {
				return err
			}
			if srv == nil {
				return nil
			}
			e := clienthttp.CatalogFile{
				MountName: mountName,
				Path:      relPath,
				Tags:      fileLatestTags(srv),
				Version:   srv.Version,
				FileHash:  srv.FileHash,
				FileSize:  srv.FileSize,
				BlobKey:   srv.BlobKey,
			}
			return pullDiscoverNew(ctx, db, api, cid, lm, e)
		}
		return err
	}
	return pullOne(ctx, db, api, cid, tf)
}
