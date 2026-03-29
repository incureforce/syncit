package cli

import (
	"errors"
	"os"
	"path/filepath"

	"go-syncit/internal/client/db"
	"go-syncit/internal/paths"
)

func clientDBPath() (string, error) {
	return paths.ClientDBPath()
}

func openClientDB() (*clientdb.ClientDB, error) {
	p, err := paths.ClientDBPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, clientdb.ErrNotConfigured
		}
		return nil, err
	}
	return clientdb.Open(p)
}

func ensureSyncitDir() (string, error) {
	d, err := paths.SyncitDir()
	if err != nil {
		return "", err
	}
	return d, os.MkdirAll(d, 0o755)
}

func expandPath(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(wd, p)), nil
}
