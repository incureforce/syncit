package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/spf13/cobra"
	clientdb "go-syncit/internal/client/db"
	clienthttp "go-syncit/internal/client/http"
	"go-syncit/internal/hashfile"
	"go-syncit/internal/mount"
)

// newPushCommand builds the "push" command (used as syncit push and syncit sync push).
func newPushCommand() *cobra.Command {
	var pushAutoAdd bool
	c := &cobra.Command{
		Use:   "push [-a] [path-or-glob...]",
		Short: "Upload local changes",
		Long: "Upload local changes to the server. Optional arguments limit the operation to those paths (within mounts). " +
			"Glob patterns are supported, including ** for recursive matches (see path/filepath and doublestar syntax).\n\n" +
			"Does not take the syncit data-directory lock so it can run while sync daemon is active; the database coordinates access.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := ensureSyncitDir(); err != nil {
				return err
			}

			db, err := openClientDB()
			if err != nil {
				return err
			}
			defer db.Close()
			ctx := cmd.Context()
			scopes, err := resolvePushScopes(ctx, db, args)
			if err != nil {
				return err
			}
			if pushAutoAdd {
				if err := autoAddChangedUnderScopes(ctx, db, scopes); err != nil {
					return err
				}
			}
			url, err := db.ServerURL(ctx)
			if err != nil {
				return err
			}
			cid, err := db.ClientID(ctx)
			if err != nil {
				return err
			}
			pending, err := db.ListNeedingPush(ctx)
			if err != nil {
				return err
			}
			pending = filterPendingByScopes(pending, scopes)
			api := &clienthttp.Client{BaseURL: url}
			for _, tf := range pending {
				root, err := db.MountRoot(ctx, tf.MountID)
				if err != nil {
					return err
				}
				localPath := filepath.Join(root, filepath.FromSlash(tf.Path))
				hash, size, err := api.UploadBlob(cid, localPath)
				if err != nil {
					return fmt.Errorf("%s: upload: %w", tf.Path, err)
				}
				if hash != tf.FileHash {
					return fmt.Errorf("%s: hash mismatch after upload", tf.Path)
				}
				mountFileID, ver, err := api.PushFile(cid, tf.MountName, clienthttp.PushFileRequest{
					Path:     tf.Path,
					Tags:     tf.Tags,
					FileHash: tf.FileHash,
					FileSize: size,
				})
				if err != nil {
					return fmt.Errorf("%s: push: %w", tf.Path, err)
				}
				_ = mountFileID
				if err := db.MarkPushed(ctx, tf.ID, ver, hash, tf.Tags); err != nil {
					return err
				}
				fmt.Printf("%s %s %s\n", syncGood("pushed"), listKey(tf.Path), listMuted(fmt.Sprintf("v%d", ver)))
			}
			if len(pending) == 0 {
				fmt.Println(listEmpty("nothing to push"))
			}
			return nil
		},
	}
	c.Flags().BoolVarP(&pushAutoAdd, "auto-add", "a", false, "re-scan tracked files on disk and update tracking when content changed (under optional paths); ignores untracked paths")
	return c
}

// pushScope limits push / auto-add to files under one mount; relPrefix uses '/' ('' = entire mount).
type pushScope struct {
	mountID   string
	mountName string
	root      string
	relPrefix string
}

func resolvePushScopes(ctx context.Context, db *clientdb.ClientDB, pathArgs []string) ([]pushScope, error) {
	mounts, err := db.ListMounts(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]mount.Entry, 0, len(mounts))
	mountByName := make(map[string]clientdb.MountRow, len(mounts))
	for _, m := range mounts {
		entries = append(entries, mount.Entry{Name: m.Name, RootPath: m.RootPath})
		mountByName[m.Name] = m
	}
	if len(pathArgs) == 0 {
		scopes := make([]pushScope, 0, len(mounts))
		for _, m := range mounts {
			scopes = append(scopes, pushScope{mountID: m.ID, mountName: m.Name, root: m.RootPath, relPrefix: ""})
		}
		return scopes, nil
	}
	var scopes []pushScope
	seen := make(map[string]struct{}, len(pathArgs))
	for _, arg := range pathArgs {
		literals, err := expandPathArgToLiterals(arg)
		if err != nil {
			return nil, err
		}
		for _, abs := range literals {
			name, root, rel, ok := mount.ResolveNearest(abs, entries)
			if !ok {
				return nil, fmt.Errorf("%s: %w", arg, errPathOutsideMounts)
			}
			m := mountByName[name]
			key := name + "\x00" + rel
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			scopes = append(scopes, pushScope{mountID: m.ID, mountName: name, root: root, relPrefix: rel})
		}
	}
	return scopes, nil
}

// pathArgHasGlobMeta reports whether s uses glob metacharacters (*, ?, [).
func pathArgHasGlobMeta(s string) bool {
	for _, r := range s {
		switch r {
		case '*', '?', '[':
			return true
		}
	}
	return false
}

// expandPathArgToLiterals returns filesystem paths for one CLI argument.
// Non-glob arguments yield a single absolute path; glob arguments are expanded
// with doublestar (supports **).
func expandPathArgToLiterals(arg string) ([]string, error) {
	absPattern, err := expandPath(arg)
	if err != nil {
		return nil, err
	}
	if !pathArgHasGlobMeta(arg) {
		return []string{absPattern}, nil
	}
	matches, err := doublestar.FilepathGlob(absPattern)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", arg, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("%s: no paths matched", arg)
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, filepath.Clean(m))
	}
	return out, nil
}

func pathUnderRelPrefix(filePath, relPrefix string) bool {
	if relPrefix == "" {
		return true
	}
	if filePath == relPrefix {
		return true
	}
	return strings.HasPrefix(filePath, relPrefix+"/")
}

func trackedMatchesScopes(tf clientdb.TrackedFile, scopes []pushScope) bool {
	for _, sc := range scopes {
		if tf.MountName != sc.mountName {
			continue
		}
		if pathUnderRelPrefix(tf.Path, sc.relPrefix) {
			return true
		}
	}
	return false
}

func filterPendingByScopes(pending []clientdb.TrackedFile, scopes []pushScope) []clientdb.TrackedFile {
	var out []clientdb.TrackedFile
	for _, tf := range pending {
		if trackedMatchesScopes(tf, scopes) {
			out = append(out, tf)
		}
	}
	return out
}

func autoAddChangedUnderScopes(ctx context.Context, db *clientdb.ClientDB, scopes []pushScope) error {
	for _, sc := range scopes {
		if err := autoAddChangedUnderScope(ctx, db, sc); err != nil {
			return err
		}
	}
	return nil
}

func autoAddChangedUnderScope(ctx context.Context, db *clientdb.ClientDB, sc pushScope) error {
	start := filepath.Join(sc.root, filepath.FromSlash(sc.relPrefix))
	st, err := os.Stat(start)
	if err != nil {
		return fmt.Errorf("%s: %w", start, err)
	}
	if !st.IsDir() {
		return addOrUpdateOneTrackedFile(ctx, db, sc, start, sc.relPrefix)
	}
	return filepath.WalkDir(start, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sc.root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		return addOrUpdateOneTrackedFile(ctx, db, sc, p, rel)
	})
}

func addOrUpdateOneTrackedFile(ctx context.Context, db *clientdb.ClientDB, sc pushScope, absPath, rel string) error {
	tf, err := db.TrackedByMountAndPath(ctx, sc.mountName, rel)
	if err != nil {
		if errors.Is(err, clientdb.ErrNotFound) {
			return nil
		}
		return err
	}
	if tf.Conflict {
		return nil
	}
	h, sz, err := hashfile.SHA256File(absPath)
	if err != nil {
		return err
	}
	if tf.FileHash == h && tf.FileSize == sz {
		return nil
	}
	return db.AddTrackedFile(ctx, sc.mountID, rel, tf.Tags, h, sz)
}
