package clienthttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PutSyncState updates the server's sync cursor for this client (RFC3339 UTC, or nil to clear).
func (c *Client) PutSyncState(clientID string, at *time.Time) error {
	var body map[string]any
	if at == nil {
		body = map[string]any{"synced_at": nil}
	} else {
		body = map[string]any{"synced_at": at.UTC().Format(time.RFC3339Nano)}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, joinURL(c.BaseURL, "/v1/client/sync-state"), bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderClientID, clientID)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("sync-state: %s: %s", resp.Status, string(b))
	}
	return nil
}
