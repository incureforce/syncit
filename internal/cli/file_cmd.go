package cli

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go-syncit/internal/hashfile"
	"go-syncit/internal/mount"
	clienthttp "go-syncit/internal/client/http"
)

func init() {
	rootCmd.AddCommand(cmdFile())
}

func cmdFile() *cobra.Command {
	c := &cobra.Command{
		Use:   "file",
		Short: "Manage tracked files",
	}
	c.AddCommand(cmdFileAdd())
	c.AddCommand(cmdFileDel())
	c.AddCommand(cmdFileGet())
	c.AddCommand(cmdFileInspect())
	c.AddCommand(cmdFileLs())
	return c
}

func cmdFileAdd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <path> [tags...]",
		Short: "Track a file or directory",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openClientDB()
			if err != nil {
				return err
			}
			defer db.Close()
			ctx := cmd.Context()
			abs, err := expandPath(args[0])
			if err != nil {
				return err
			}
			fileTags := args[1:]
			mounts, err := db.ListMounts(ctx)
			if err != nil {
				return err
			}
			entries := make([]mount.Entry, 0, len(mounts))
			for _, m := range mounts {
				entries = append(entries, mount.Entry{Name: m.Name, RootPath: m.RootPath})
			}
			name, _, relPrefix, ok := mount.ResolveNearest(abs, entries)
			if !ok {
				return errPathOutsideMounts
			}
			mr, err := db.MountByName(ctx, name)
			if err != nil {
				return err
			}
			st, err := os.Stat(abs)
			if err != nil {
				return err
			}
			if st.IsDir() {
				return filepath.WalkDir(abs, func(p string, d fs.DirEntry, walkErr error) error {
					if walkErr != nil {
						return walkErr
					}
					if d.IsDir() {
						return nil
					}
					rel, err := filepath.Rel(mr.RootPath, p)
					if err != nil {
						return err
					}
					rel = filepath.ToSlash(rel)
					h, sz, err := hashfile.SHA256File(p)
					if err != nil {
						return err
					}
					return db.AddTrackedFile(ctx, mr.ID, rel, fileTags, h, sz)
				})
			}
			rel := relPrefix
			if rel == "" {
				rel = filepath.Base(abs)
			}
			h, sz, err := hashfile.SHA256File(abs)
			if err != nil {
				return err
			}
			return db.AddTrackedFile(ctx, mr.ID, rel, fileTags, h, sz)
		},
	}
}

var errPathOutsideMounts = errOutsideMounts{}

type errOutsideMounts struct{}

func (errOutsideMounts) Error() string {
	return "path is outside all mount points"
}

func cmdFileDel() *cobra.Command {
	return &cobra.Command{
		Use:   "del <path> [tag]",
		Short: "Remove tracking or delete remotely",
		Run: func(cmd *cobra.Command, args []string) {
			exitNotImplemented("file del")
		},
	}
}

func cmdFileGet() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "get <path>",
		Short: "Pull remote content for a path (remote wins)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openClientDB()
			if err != nil {
				return err
			}
			defer db.Close()
			ctx := cmd.Context()
			tf, err := resolveToTracked(db, ctx, args[0])
			if err != nil {
				return err
			}
			url, err := db.ServerURL(ctx)
			if err != nil {
				return err
			}
			cid, err := db.ClientID(ctx)
			if err != nil {
				return err
			}
			api := &clienthttp.Client{BaseURL: url}
			srv, err := api.GetFileLatest(cid, tf.MountName, tf.Path)
			if err != nil {
				return err
			}
			if srv == nil {
				return fmt.Errorf("no remote file for this path")
			}
			root, err := db.MountRoot(ctx, tf.MountID)
			if err != nil {
				return err
			}
			localPath := filepath.Join(root, filepath.FromSlash(tf.Path))
			if st, err := os.Stat(localPath); err == nil && !st.IsDir() {
				h, _, err := hashfile.SHA256File(localPath)
				if err != nil {
					return err
				}
				if h != srv.FileHash && !yes {
					fmt.Fprintf(os.Stderr, "Local file differs from remote. Overwrite? [y/N]: ")
					line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
					if strings.TrimSpace(strings.ToLower(line)) != "y" {
						return fmt.Errorf("cancelled")
					}
				}
			}
			if err := api.DownloadBlobToFile(srv.BlobKey, localPath); err != nil {
				return err
			}
			srvTags := srv.Tags
			if srvTags == nil {
				srvTags = []string{}
			}
			if err := db.ApplyPull(ctx, tf.ID, srv.FileHash, srv.Version, srvTags); err != nil {
				return err
			}
			fmt.Printf("Updated %s from server (v%d)\n", tf.Path, srv.Version)
			return nil
		},
	}
	c.Flags().BoolVarP(&yes, "yes", "y", false, "Do not prompt before overwriting")
	return c
}

func cmdFileInspect() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <path>",
		Short: "Show status and history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openClientDB()
			if err != nil {
				return err
			}
			defer db.Close()
			ctx := cmd.Context()
			tf, err := resolveToTracked(db, ctx, args[0])
			if err != nil {
				return err
			}
			st := trackedLineStatus(tf)
			fmt.Printf("%s\n", st.Paint(st.Label))
			fmt.Printf("  mount: %s\n", tf.MountName)
			fmt.Printf("  path:  %s\n", tf.Path)
			fmt.Printf("  tags:  %v\n", tf.Tags)
			fmt.Printf("  local version:  %d\n", tf.Version)
			fmt.Printf("  remote version: %d\n", tf.RemoteVersion)
			fmt.Printf("  file_hash:   %s\n", shortHash(tf.FileHash))
			fmt.Printf("  remote_hash: %s\n", shortHash(tf.RemoteHash))
			fmt.Printf("  size: %d\n", tf.FileSize)
			return nil
		},
	}
}

func cmdFileLs() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List tracked files (status, path, tags, local v, remote rv)",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openClientDB()
			if err != nil {
				return err
			}
			defer db.Close()
			list, err := db.ListTrackedWithMount(cmd.Context())
			if err != nil {
				return err
			}
			if len(list) == 0 {
				fmt.Println(listEmpty("no tracked files"))
				return nil
			}
			for _, tf := range list {
				st := trackedLineStatus(tf)
				loc := listKey(tf.MountName + ":" + tf.Path)
				tagsCol := listMuted("-")
				if len(tf.Tags) > 0 {
					tagsCol = listTag(strings.Join(tf.Tags, ", "))
				}
				vCol := listMuted(fmt.Sprintf("v%d", tf.Version))
				rvCol := listMuted(fmt.Sprintf("rv%d", tf.RemoteVersion))
				if tf.Conflict {
					vCol = syncBad(fmt.Sprintf("v%d", tf.Version))
					rvCol = syncBad(fmt.Sprintf("rv%d", tf.RemoteVersion))
				}
				fmt.Printf("%s  %s  %s  %s  %s\n",
					st.Paint(st.Label),
					loc,
					tagsCol,
					vCol,
					rvCol,
				)
			}
			return nil
		},
	}
}
