package lock

import (
	"path/filepath"
	"testing"
)

func TestTryLockDirContention(t *testing.T) {
	dir := t.TempDir()
	a, err := TryLockDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	_, err = TryLockDir(dir)
	if err == nil {
		t.Fatal("expected second lock to fail")
	}
	if !IsErrHeld(err) {
		t.Fatalf("expected ErrHeld, got %v", err)
	}
}

func TestTryLockSeparateDirs(t *testing.T) {
	d1 := filepath.Join(t.TempDir(), "a")
	d2 := filepath.Join(t.TempDir(), "b")
	a, err := TryLockDir(d1)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := TryLockDir(d2)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
}
