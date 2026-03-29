package clienthttp

import (
	"io"
	"os"
	"path/filepath"
)

// DownloadBlobToFile downloads a blob to a path (creates parent directories).
func (c *Client) DownloadBlobToFile(hash, destPath string) error {
	r, err := c.DownloadBlob(hash)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	return f.Close()
}
