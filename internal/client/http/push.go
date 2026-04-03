package clienthttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// UploadBlob streams a file into server blob storage; returns content hash and size.
func (c *Client) UploadBlob(clientID string, filePath string) (hash string, size int64, err error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	req, err := http.NewRequest(http.MethodPost, joinURL(c.BaseURL, "/v1/blobs"), f)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set(HeaderClientID, clientID)
	req.ContentLength = st.Size()
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", 0, fmt.Errorf("upload blob: %s: %s", resp.Status, string(b))
	}
	var out struct {
		Hash string `json:"hash"`
		Size int64  `json:"size"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", 0, err
	}
	return out.Hash, out.Size, nil
}

// PushFileRequest is the JSON body for POST /v1/mounts/{mount}/files.
type PushFileRequest struct {
	Path     string   `json:"path"`
	Tags     []string `json:"tags"`
	FileHash string   `json:"file_hash"`
	FileSize int64    `json:"file_size"`
}

// PushFile records a new file version on the server (after blob upload).
func (c *Client) PushFile(clientID, mountName string, req PushFileRequest) (mountFileID string, version int, err error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", 0, err
	}
	u := mountFilesPrefix(c.BaseURL, mountName)
	r, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(HeaderClientID, clientID)
	resp, err := c.httpClient().Do(r)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", 0, fmt.Errorf("push file: %s: %s", resp.Status, string(b))
	}
	var out struct {
		MountFileID string `json:"mount_file_id"`
		Version     int    `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", 0, err
	}
	return out.MountFileID, out.Version, nil
}

// DownloadBlob fetches blob bytes by hash (GET /v1/blobs/{hash}).
func (c *Client) DownloadBlob(hash string) (io.ReadCloser, error) {
	u := joinURL(c.BaseURL, "/v1/blobs/"+strings.TrimSpace(hash))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("download blob: %s", resp.Status)
	}
	return resp.Body, nil
}

// DeleteFile marks a server file entry deleted for the given mount/path.
// Returns true when a remote row changed, false when it was already deleted or absent.
func (c *Client) DeleteFile(clientID, mountName, relPath string) (bool, error) {
	u := mountFileItemURL(c.BaseURL, mountName, relPath)
	req, err := http.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set(HeaderClientID, clientID)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return false, fmt.Errorf("delete file: %s: %s", resp.Status, string(b))
	}
	var out struct {
		Deleted bool `json:"deleted"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, err
	}
	return out.Deleted, nil
}
