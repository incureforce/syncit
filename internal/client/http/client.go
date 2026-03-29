package clienthttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HeaderClientID is sent on every API request identifying this client.
const HeaderClientID = "X-Syncit-Client-ID"

// Client wraps HTTP calls to the syncit server.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func joinURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	return base + path
}

// mountFilesPrefix is GET/POST .../v1/mounts/{mount}/files (no trailing path segment).
func mountFilesPrefix(baseURL, mount string) string {
	return joinURL(baseURL, "/v1/mounts/"+url.PathEscape(mount)+"/files")
}

// mountFileItemURL is GET .../v1/mounts/{mount}/files/{relPath}.
func mountFileItemURL(baseURL, mount, relPath string) string {
	u := mountFilesPrefix(baseURL, mount)
	relPath = strings.Trim(relPath, "/")
	if relPath == "" {
		return u
	}
	for _, p := range strings.Split(relPath, "/") {
		if p != "" {
			u += "/" + url.PathEscape(p)
		}
	}
	return u
}

// Register registers or refreshes this client on the server.
func (c *Client) Register(clientID, name string) error {
	body, err := json.Marshal(map[string]string{"id": clientID, "name": name})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, joinURL(c.BaseURL, "/v1/clients/register"), bytes.NewReader(body))
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
		return fmt.Errorf("register: %s: %s", resp.Status, string(b))
	}
	return nil
}

// SetClientTags updates tags on the server for this client.
func (c *Client) SetClientTags(clientID string, tags []string) error {
	body, err := json.Marshal(map[string][]string{"tags": tags})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, joinURL(c.BaseURL, "/v1/client/tags"), bytes.NewReader(body))
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
		return fmt.Errorf("tags: %s: %s", resp.Status, string(b))
	}
	return nil
}
