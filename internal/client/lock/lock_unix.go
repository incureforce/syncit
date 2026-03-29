//go:build !windows

package lock

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func tryLockFile(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return ErrHeld
		}
		return err
	}
	return nil
}

func unlockFile(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
