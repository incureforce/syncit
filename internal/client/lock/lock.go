package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrHeld is returned when another process holds the lock.
var ErrHeld = errors.New("syncit data directory is in use by another process (sync daemon or sync/mount/tag command)")

// IsErrHeld reports whether err indicates the data directory lock is held.
func IsErrHeld(err error) bool {
	return errors.Is(err, ErrHeld)
}

// FileLock is an exclusive lock on the syncit data directory.
type FileLock struct {
	path string
	f    *os.File
}

// TryLockDir acquires an exclusive lock on syncit.lock under dir, or returns ErrHeld.
func TryLockDir(dir string) (*FileLock, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	p := filepath.Join(dir, "syncit.lock")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := tryLockFile(f); err != nil {
		_ = f.Close()
		if errors.Is(err, ErrHeld) {
			return nil, fmt.Errorf("%w: %s", ErrHeld, p)
		}
		return nil, err
	}
	return &FileLock{path: p, f: f}, nil
}

// Close releases the lock and closes the file.
func (l *FileLock) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	err := unlockFile(l.f)
	c2 := l.f.Close()
	l.f = nil
	if err != nil {
		return err
	}
	return c2
}
