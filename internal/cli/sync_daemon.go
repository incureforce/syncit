package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	clientdb "go-syncit/internal/client/db"
	clienthttp "go-syncit/internal/client/http"
	"go-syncit/internal/client/lock"
)

func cmdSyncDaemon() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Stay in sync using server push notifications (SSE)",
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

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			url, err := db.ServerURL(ctx)
			if err != nil {
				return err
			}
			cid, err := db.ClientID(ctx)
			if err != nil {
				return err
			}
			api := &clienthttp.Client{
				BaseURL:    url,
				HTTPClient: &http.Client{Timeout: 0},
			}
			return runSyncDaemon(ctx, db, api, cid)
		},
	}
}

func runSyncDaemon(ctx context.Context, db *clientdb.ClientDB, api *clienthttp.Client, cid string) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, ok, err := db.GetSyncedAt(ctx)
		if err != nil {
			return err
		}
		if !ok {
			if err := RunPullFull(ctx, db, api, cid); err != nil {
				return err
			}
			if err := AdvanceSyncState(ctx, db, api, cid); err != nil {
				return err
			}
		}

		err = api.StreamEvents(ctx, cid, func(data []byte) error {
			var ev struct {
				Kind  string `json:"kind"`
				Mount string `json:"mount"`
				Path  string `json:"path"`
			}
			if err := json.Unmarshal(data, &ev); err != nil {
				return nil
			}
			if ev.Kind != "file_version" {
				return nil
			}
			if err := PullPathAfterEvent(ctx, db, api, cid, ev.Mount, ev.Path); err != nil {
				return err
			}
			return AdvanceSyncState(ctx, db, api, cid)
		})
		if err := ctx.Err(); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "event stream ended (%v); running full pull\n", err)
		if err := RunPullFull(ctx, db, api, cid); err != nil {
			return err
		}
		if err := AdvanceSyncState(ctx, db, api, cid); err != nil {
			return err
		}
	}
}
