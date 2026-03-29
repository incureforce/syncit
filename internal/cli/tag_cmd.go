package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	clienthttp "go-syncit/internal/client/http"
	"go-syncit/internal/client/lock"
)

func init() {
	rootCmd.AddCommand(cmdTag())
}

func cmdTag() *cobra.Command {
	c := &cobra.Command{
		Use:   "tag",
		Short: "Manage client tags",
	}
	c.AddCommand(
		&cobra.Command{
			Use:   "ls",
			Short: "List this client's tags",
			RunE: func(cmd *cobra.Command, args []string) error {
				db, err := openClientDB()
				if err != nil {
					return err
				}
				defer db.Close()
				tags, err := db.Tags(cmd.Context())
				if err != nil {
					return err
				}
				if len(tags) == 0 {
					fmt.Println(listEmpty("no tags"))
					return nil
				}
				for _, t := range tags {
					fmt.Println(listTag(t))
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "add <tag> [more-tags...]",
			Short: "Add tags to this client",
			Args:  cobra.MinimumNArgs(1),
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
				if err := db.AddTags(ctx, args); err != nil {
					return err
				}
				tags, err := db.Tags(ctx)
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
				if err := api.SetClientTags(cid, tags); err != nil {
					return fmt.Errorf("sync tags to server: %w", err)
				}
				if err := db.ClearSyncedAt(ctx); err != nil {
					return err
				}
				fmt.Printf("Tags: %v\n", tags)
				return nil
			},
		},
		&cobra.Command{
			Use:   "del <tag> [more-tags...]",
			Short: "Remove tags from this client",
			Args:  cobra.MinimumNArgs(1),
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
				if err := db.RemoveTags(ctx, args); err != nil {
					return err
				}
				tags, err := db.Tags(ctx)
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
				if err := api.SetClientTags(cid, tags); err != nil {
					return fmt.Errorf("sync tags to server: %w", err)
				}
				if err := db.ClearSyncedAt(ctx); err != nil {
					return err
				}
				fmt.Printf("Tags: %v\n", tags)
				return nil
			},
		},
	)
	return c
}
