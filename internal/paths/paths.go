package paths

import (
	"os"
	"path/filepath"
)

// SyncitDir returns the directory holding client.db and local state.
func SyncitDir() (string, error) {
	if d := os.Getenv("SYNCIT_HOME"); d != "" {
		return filepath.Clean(d), nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".syncit"), nil
}

// ClientDBPath is the SQLite path for the client.
func ClientDBPath() (string, error) {
	dir, err := SyncitDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "client.db"), nil
}
