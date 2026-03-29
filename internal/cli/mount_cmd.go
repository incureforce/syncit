package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	clienthttp "go-syncit/internal/client/http"
	"go-syncit/internal/client/lock"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cmdMount())
}

const unmappedPath = "(unmapped)"

func cmdMount() *cobra.Command {
	c := &cobra.Command{
		Use:   "mount",
		Short: "Manage mount points",
	}
	lsCmd := &cobra.Command{
		Use:   "ls",
		Short: "List mounts (local paths; -a adds server ids and unmapped server mounts)",
		RunE: func(cmd *cobra.Command, args []string) error {
			all, err := cmd.Flags().GetBool("all")
			if err != nil {
				return err
			}
			db, err := openClientDB()
			if err != nil {
				return err
			}
			defer db.Close()
			ctx := cmd.Context()

			localMounts, err := db.ListMounts(ctx)
			if err != nil {
				return err
			}
			localByName := make(map[string]string, len(localMounts))
			for _, m := range localMounts {
				localByName[m.Name] = m.RootPath
			}

			if !all {
				if len(localMounts) == 0 {
					fmt.Println(listEmpty("no local mounts"))
					return nil
				}
				for _, m := range localMounts {
					fmt.Printf("%s  %s\n", listKey(m.Name), m.RootPath)
				}
				return nil
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
			remote, err := api.ListRemoteMounts(cid)
			if err != nil {
				return err
			}

			seen := make(map[string]struct{})
			type row struct {
				name, id, path string
			}
			var rows []row
			for _, r := range remote {
				seen[r.Name] = struct{}{}
				path := unmappedPath
				if p, ok := localByName[r.Name]; ok {
					path = p
				}
				rows = append(rows, row{name: r.Name, id: r.ID, path: path})
			}
			for name, p := range localByName {
				if _, ok := seen[name]; ok {
					continue
				}
				rows = append(rows, row{name: name, id: "-", path: p})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

			if len(rows) == 0 {
				fmt.Println(listEmpty("no mounts"))
				return nil
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			for _, r := range rows {
				pathCol := strings.ReplaceAll(r.path, "\t", " ")
				if r.path == unmappedPath {
					pathCol = listWarn(pathCol)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", listKey(r.name), listMuted(r.id), pathCol)
			}
			_ = tw.Flush()
			return nil
		},
	}
	lsCmd.Flags().BoolP("all", "a", false, "list server mount ids with local paths; server-only mounts show (unmapped) path")
	c.AddCommand(
		lsCmd,
		&cobra.Command{
			Use:   "add <name> <path>",
			Short: "Add a named mount",
			Args:  cobra.ExactArgs(2),
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
				path, err := expandPath(args[1])
				if err != nil {
					return err
				}
				ctx := cmd.Context()
				if err := db.AddMount(ctx, args[0], path); err != nil {
					return err
				}
				if err := pushLocalMountsToServer(ctx, db); err != nil {
					return fmt.Errorf("update server mounts: %w", err)
				}
				if err := db.ClearSyncedAt(ctx); err != nil {
					return err
				}
				fmt.Printf("Added mount %q -> %s\n", args[0], path)
				return nil
			},
		},
		&cobra.Command{
			Use:   "del <name>",
			Short: "Remove a mount",
			Args:  cobra.ExactArgs(1),
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
				if err := db.DeleteMount(ctx, args[0]); err != nil {
					return err
				}
				if err := pushLocalMountsToServer(ctx, db); err != nil {
					return fmt.Errorf("update server mounts: %w", err)
				}
				if err := db.ClearSyncedAt(ctx); err != nil {
					return err
				}
				fmt.Printf("Removed mount %q\n", args[0])
				return nil
			},
		},
	)
	return c
}
