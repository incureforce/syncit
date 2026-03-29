package cli

import (
	"context"
	"fmt"
	serverhttp "go-syncit/internal/server/http"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	runAddr    string
	runDataDir string
)

func init() {
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Start the syncit HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer()
		},
	}
	runCmd.Flags().StringVar(&runAddr, "addr", ":8080", "HTTP listen address")
	runCmd.Flags().StringVar(&runDataDir, "data-dir", "./syncit-data", "Directory for SQLite DB and blob store")
	rootCmd.AddCommand(runCmd)
}

func runServer() error {
	srv, err := serverhttp.New(serverhttp.Config{Addr: runAddr, DataDir: runDataDir})
	if err != nil {
		return err
	}
	defer srv.Close()
	httpSrv := &http.Server{
		Addr:              runAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		fmt.Fprintf(os.Stderr, "syncit server listening on %s\n", runAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server: %v\n", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpSrv.Shutdown(shutdownCtx)
}
