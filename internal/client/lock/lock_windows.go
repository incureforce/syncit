//go:build windows

package lock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func tryLockFile(f *os.File) error {
	var o windows.Overlapped
	h := windows.Handle(f.Fd())
	err := windows.LockFileEx(h,
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, &o)
	if err != nil {
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return ErrHeld
		}
		return err
	}
	return nil
}

func unlockFile(f *os.File) error {
	var o windows.Overlapped
	h := windows.Handle(f.Fd())
	return windows.UnlockFileEx(h, 0, 1, 0, &o)
}
