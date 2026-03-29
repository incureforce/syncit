package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	clientdb "go-syncit/internal/client/db"
	clienthttp "go-syncit/internal/client/http"
)

func init() {
	rootCmd.AddCommand(cmdInit())
}

func cmdInit() *cobra.Command {
	return &cobra.Command{
		Use:   "init <server-url>",
		Short: "Initialize this client against the server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serverURL := args[0]
			dir, err := ensureSyncitDir()
			if err != nil {
				return err
			}
			dbPath := filepath.Join(dir, "client.db")
			if _, err := os.Stat(dbPath); err == nil {
				return fmt.Errorf("already initialized at %s", dbPath)
			}

			db, err := clientdb.InitNew(cmd.Context(), dbPath, serverURL)
			if err != nil {
				return err
			}

			// If anything fails after InitNew commits, drop client.db so the user can retry init.
			partial := true
			defer func() {
				_ = db.Close()
				if partial {
					if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
						fmt.Fprintf(os.Stderr, "syncit: could not remove partial database %s: %v\n", dbPath, err)
					}
				}
			}()

			cid, err := db.ClientID(cmd.Context())
			if err != nil {
				return err
			}
			api := &clienthttp.Client{BaseURL: serverURL}
			if err := api.Register(cid, ""); err != nil {
				return fmt.Errorf("register with server: %w", err)
			}
			partial = false
			fmt.Printf("Initialized syncit client %s\nServer: %s\nData: %s\n(add mounts with: syncit mount add <name> <path>)\n", cid, serverURL, dir)
			return nil
		},
	}
}
