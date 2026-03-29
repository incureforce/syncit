package cli

import (
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cmdShare())
}

func cmdShare() *cobra.Command {
	c := &cobra.Command{
		Use:   "share",
		Short: "Share files (optional / phase 2)",
	}
	c.AddCommand(
		&cobra.Command{
			Use:   "ls",
			Short: "List shares",
			Run: func(cmd *cobra.Command, args []string) {
				_, _ = color.New(color.FgYellow).Fprintln(os.Stderr, "share ls: not implemented in v1 (phase 2)")
				os.Exit(0)
			},
		},
		&cobra.Command{
			Use:   "put <name> <path>",
			Short: "Create a share",
			Args:  cobra.ExactArgs(2),
			Run: func(cmd *cobra.Command, args []string) {
				_, _ = color.New(color.FgYellow).Fprintln(os.Stderr, "share put: not implemented in v1 (phase 2)")
				os.Exit(0)
			},
		},
		&cobra.Command{
			Use:   "get <id>",
			Short: "Download a share",
			Args:  cobra.ExactArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				_, _ = color.New(color.FgYellow).Fprintln(os.Stderr, "share get: not implemented in v1 (phase 2)")
				os.Exit(0)
			},
		},
	)
	return c
}
