package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go-syncit/internal/paths"
)

func init() {
	rootCmd.AddCommand(cmdWipe())
}

func cmdWipe() *cobra.Command {
	return &cobra.Command{
		Use:   "wipe",
		Short: "Disconnect this client and remove all local syncit files",
		Long:  "Remove the ~/.syncit directory, disconnecting this client from the server and deleting all local syncit state.",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := paths.SyncitDir()
			if err != nil {
				return err
			}

			// Confirm before wiping
			fmt.Printf("This will remove: %s\n", dir)
			fmt.Print("Are you sure? Type 'yes' to confirm: ")
			var response string
			_, err = fmt.Scanln(&response)
			if err != nil {
				return fmt.Errorf("failed to read response: %w", err)
			}

			if response != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}

			// Remove the directory
			if err := os.RemoveAll(dir); err != nil {
				return fmt.Errorf("failed to remove %s: %w", dir, err)
			}

			fmt.Printf("Successfully wiped %s\n", dir)
			return nil
		},
	}
}
