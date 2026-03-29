package blob

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Store is a content-addressed blob directory (SHA-256 keys).
type Store struct {
	root string
}

// New creates a blob store under rootDir (typically dataDir/blobs).
func New(rootDir string) (*Store, error) {
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, err
	}
	return &Store{root: rootDir}, nil
}

// PathForKey returns the filesystem path for a hex-encoded SHA-256 key.
func (s *Store) PathForKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	if len(key) < 4 {
		return filepath.Join(s.root, key)
	}
	return filepath.Join(s.root, key[:2], key[2:4], key)
}

// Has reports whether a blob exists.
func (s *Store) Has(key string) bool {
	st, err := os.Stat(s.PathForKey(key))
	return err == nil && !st.IsDir()
}

// Put writes content from r, returns hex SHA-256 key, byte size, and whether a new blob file was stored.
func (s *Store) Put(r io.Reader) (key string, n int64, isNew bool, err error) {
	h := sha256.New()
	tr := io.TeeReader(r, h)
	f, err := os.CreateTemp(s.root, "upload-*")
	if err != nil {
		return "", 0, false, err
	}
	tmpPath := f.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	written, err := io.Copy(f, tr)
	if err != nil {
		_ = f.Close()
		return "", 0, false, err
	}
	if err := f.Close(); err != nil {
		return "", 0, false, err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	dest := s.PathForKey(sum)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", 0, false, err
	}
	if s.Has(sum) {
		return sum, written, false, nil
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		if s.Has(sum) {
			return sum, written, false, nil
		}
		return "", 0, false, err
	}
	return sum, written, true, nil
}

// Open opens a blob for reading.
func (s *Store) Open(key string) (*os.File, error) {
	return os.Open(s.PathForKey(key))
}

// Remove deletes a stored blob if present.
func (s *Store) Remove(key string) error {
	p := s.PathForKey(key)
	err := os.Remove(p)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// WriteFile writes bytes and returns the content hash (for small blobs).
func (s *Store) WriteFile(data []byte) (key string, err error) {
	k, _, _, err := s.Put(bytes.NewReader(data))
	return k, err
}
