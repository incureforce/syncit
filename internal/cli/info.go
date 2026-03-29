package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cmdInfo())
}

func cmdInfo() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show mounts and tags",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			tags, err := db.Tags(ctx)
			if err != nil {
				return err
			}
			mounts, err := db.ListMounts(ctx)
			if err != nil {
				return err
			}
			fmt.Printf("Server: %s\n", url)
			fmt.Printf("Tags: %v\n", tags)
			fmt.Println("Mounts:")
			for _, m := range mounts {
				fmt.Printf("  %s  %s\n", m.Name, m.RootPath)
			}
			return nil
		},
	}
}
