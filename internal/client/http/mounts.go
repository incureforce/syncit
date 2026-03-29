package clienthttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// RemoteMount is one mount row from GET /v1/mounts.
type RemoteMount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PutClientMounts replaces the server's mount links for this client with the given names (add/remove client_mounts).
func (c *Client) PutClientMounts(clientID string, mountNames []string) error {
	body, err := json.Marshal(map[string]any{"mounts": mountNames})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, joinURL(c.BaseURL, "/v1/client/mounts"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set(HeaderClientID, clientID)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("client/mounts: %s: %s", resp.Status, string(b))
	}
	return nil
}

// ListRemoteMounts returns all global mount names on the server (GET /v1/mounts).
func (c *Client) ListRemoteMounts(clientID string) ([]RemoteMount, error) {
	req, err := http.NewRequest(http.MethodGet, joinURL(c.BaseURL, "/v1/mounts"), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(HeaderClientID, clientID)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("mounts: %s: %s", resp.Status, string(b))
	}
	var out struct {
		Mounts []RemoteMount `json:"mounts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Mounts, nil
}
