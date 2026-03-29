package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go-syncit/internal/client/db"
)

var rootCmd = &cobra.Command{
	Use:   "syncit",
	Short: "Sync files with a shared syncit server",
	Long:  "Client and server commands for syncit (see docs/design_v1.md).",
	SilenceUsage: true,
}

// Execute runs the root command.
func Execute() error {
	err := rootCmd.Execute()
	if err == nil {
		return nil
	}
	if errors.Is(err, clientdb.ErrNotConfigured) {
		fmt.Fprintln(os.Stderr, "syncit is not initialized; run: syncit init <server-url>")
	}
	return err
}

func exitNotImplemented(cmd string) {
	fmt.Fprintf(os.Stderr, "syncit %s: not implemented yet\n", cmd)
	os.Exit(0)
}
