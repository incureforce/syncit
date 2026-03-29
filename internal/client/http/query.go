package clienthttp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// FileLatest is the server JSON for GET /v1/mounts/{mount}/files/{path}.
type FileLatest struct {
	Version   int      `json:"version"`
	FileHash  string   `json:"file_hash"`
	FileSize  int64    `json:"file_size"`
	BlobKey   string   `json:"blob_key"`
	MountName string   `json:"mount_name"`
	Path      string   `json:"path"`
	Tags      []string `json:"tags"`
}

// GetFileLatest returns remote metadata or (nil, nil) if not found / not visible.
func (c *Client) GetFileLatest(clientID, mountName, relPath string) (*FileLatest, error) {
	u := mountFileItemURL(c.BaseURL, mountName, relPath)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(HeaderClientID, clientID)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("file/latest: %s: %s", resp.Status, string(b))
	}
	var out FileLatest
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CatalogFile is one row from GET /v1/mounts/{mount}/files.
type CatalogFile struct {
	MountName string   `json:"mount_name"`
	Path      string   `json:"path"`
	Tags      []string `json:"tags"`
	Version   int      `json:"version"`
	FileHash  string   `json:"file_hash"`
	FileSize  int64    `json:"file_size"`
	BlobKey   string   `json:"blob_key"`
}

// ListFilesLatest returns visible files for one mount (GET /v1/mounts/{mount}/files).
func (c *Client) ListFilesLatest(clientID, mountName string) ([]CatalogFile, error) {
	req, err := http.NewRequest(http.MethodGet, mountFilesPrefix(c.BaseURL, mountName), nil)
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
		return nil, fmt.Errorf("files/latest: %s: %s", resp.Status, string(b))
	}
	var out struct {
		Files []CatalogFile `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Files, nil
}
