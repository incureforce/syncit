package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	clientdb "go-syncit/internal/client/db"
	clienthttp "go-syncit/internal/client/http"
	"go-syncit/internal/client/lock"
	"go-syncit/internal/hashfile"
	"go-syncit/internal/tags"
)

func init() {
	rootCmd.AddCommand(cmdSync())
	rootCmd.AddCommand(newPushCommand())
}

func cmdSync() *cobra.Command {
	c := &cobra.Command{
		Use:   "sync",
		Short: "Manual sync with the server",
	}
	c.AddCommand(cmdSyncPush())
	c.AddCommand(cmdSyncPull())
	c.AddCommand(cmdSyncDaemon())
	return c
}

func cmdSyncPush() *cobra.Command {
	return newPushCommand()
}

func cmdSyncPull() *cobra.Command {
	return &cobra.Command{
		Use:   "pull",
		Short: "Download remote changes (full catalog; ignores sync cursor — use to repair)",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := ensureSyncitDir()
			if err != nil {
				return err
			}
			lk, err := lock.TryLockDir(dir)
			if err != nil {
				return err
			}
			defer lk.Close()

			db, err := openClientDB()
			if err != nil {
				return err
			}
			defer db.Close()
			ctx := cmd.Context()
			url, err := db.ServerURL(ctx)
			if err != nil {
				return err
			}
			cid, err := db.ClientID(ctx)
			if err != nil {
				return err
			}
			api := &clienthttp.Client{BaseURL: url}
			if err := RunPullFull(ctx, db, api, cid); err != nil {
				return err
			}
			return AdvanceSyncState(ctx, db, api, cid)
		},
	}
}

func pullTrackKey(mountName, relPath string) string {
	return mountName + "\x00" + relPath
}

func fileLatestTags(s *clienthttp.FileLatest) []string {
	if s == nil || s.Tags == nil {
		return []string{}
	}
	return s.Tags
}

func syncServerMounts(ctx context.Context, db *clientdb.ClientDB, api *clienthttp.Client, cid string) error {
	mounts, err := db.ListMounts(ctx)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(mounts))
	for _, m := range mounts {
		names = append(names, m.Name)
	}
	return api.PutClientMounts(cid, names)
}

// pushLocalMountsToServer sends the current local mount names (PUT /v1/client/mounts; replaces server links).
func pushLocalMountsToServer(ctx context.Context, db *clientdb.ClientDB) error {
	url, err := db.ServerURL(ctx)
	if err != nil {
		return err
	}
	cid, err := db.ClientID(ctx)
	if err != nil {
		return err
	}
	api := &clienthttp.Client{BaseURL: url}
	return syncServerMounts(ctx, db, api, cid)
}

func pullDiscoverNew(ctx context.Context, db *clientdb.ClientDB, api *clienthttp.Client, cid string, m clientdb.MountRow, e clienthttp.CatalogFile) error {
	_ = cid
	localPath := filepath.Join(m.RootPath, filepath.FromSlash(e.Path))
	if err := api.DownloadBlobToFile(e.BlobKey, localPath); err != nil {
		return fmt.Errorf("%s:%s: download: %w", e.MountName, e.Path, err)
	}
	h, sz, err := hashfile.SHA256File(localPath)
	if err != nil {
		return fmt.Errorf("%s:%s: hash: %w", e.MountName, e.Path, err)
	}
	if !strings.EqualFold(h, e.FileHash) {
		return fmt.Errorf("%s:%s: checksum mismatch after download", e.MountName, e.Path)
	}
	if sz != e.FileSize {
		return fmt.Errorf("%s:%s: size mismatch after download (got %d want %d)", e.MountName, e.Path, sz, e.FileSize)
	}
	inserted, err := db.InsertNewFromRemotePull(ctx, m.ID, e.Path, e.Tags, e.FileHash, e.FileSize, e.Version, e.BlobKey)
	if err != nil {
		return err
	}
	if inserted {
		fmt.Printf("%s %s\n", syncGood("discovered"), listKey(e.MountName+":"+e.Path))
	}
	return nil
}

func pullOne(ctx context.Context, db *clientdb.ClientDB, api *clienthttp.Client, cid string, tf clientdb.TrackedFile) error {
	srv, err := api.GetFileLatest(cid, tf.MountName, tf.Path)
	if err != nil {
		return err
	}
	if srv == nil {
		return nil
	}

	srvTags := fileLatestTags(srv)
	if srv.FileHash == tf.FileHash {
		if tf.RemoteHash != srv.FileHash || tf.RemoteVersion != srv.Version || !tags.Equal(tf.Tags, srvTags) {
			return db.ApplyPull(ctx, tf.ID, srv.FileHash, srv.Version, srvTags)
		}
		return nil
	}
	if tf.FileHash == tf.RemoteHash {
		root, err := db.MountRoot(ctx, tf.MountID)
		if err != nil {
			return err
		}
		localPath := filepath.Join(root, filepath.FromSlash(tf.Path))
		if err := api.DownloadBlobToFile(srv.BlobKey, localPath); err != nil {
			return fmt.Errorf("%s: download: %w", tf.Path, err)
		}
		if err := db.ApplyPull(ctx, tf.ID, srv.FileHash, srv.Version, srvTags); err != nil {
			return err
		}
		fmt.Printf("%s %s %s\n", syncGood("pulled"), listKey(tf.Path), listMuted(fmt.Sprintf("v%d", srv.Version)))
		return nil
	}
	if srv.FileHash == tf.RemoteHash {
		if tf.FileHash != srv.FileHash {
			if err := db.SetConflict(ctx, tf.ID, true); err != nil {
				return err
			}
			fmt.Printf("%s %s (resolve with: syncit file get -y %q)\n", syncBad("conflict:"), listKey(tf.Path), tf.Path)
			return nil
		}
		return nil
	}
	if err := db.SetConflict(ctx, tf.ID, true); err != nil {
		return err
	}
	fmt.Printf("%s %s (resolve with: syncit file get -y %q)\n", syncBad("conflict:"), listKey(tf.Path), tf.Path)
	return nil
}
