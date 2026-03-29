package cli

import (
	"os"
	"path/filepath"
	"testing"

	clientdb "go-syncit/internal/client/db"
)

func TestPathUnderRelPrefix(t *testing.T) {
	tests := []struct {
		file, prefix string
		want           bool
	}{
		{"a/b", "", true},
		{"a/b", "a", true},
		{"a/b", "a/b", true},
		{"a/b", "a/c", false},
		{"a", "a/b", false},
		{"ab", "a", false},
	}
	for _, tc := range tests {
		if got := pathUnderRelPrefix(tc.file, tc.prefix); got != tc.want {
			t.Errorf("pathUnderRelPrefix(%q, %q) = %v, want %v", tc.file, tc.prefix, got, tc.want)
		}
	}
}

func TestPathArgHasGlobMeta(t *testing.T) {
	if !pathArgHasGlobMeta("*.go") || !pathArgHasGlobMeta("a[bc]") || !pathArgHasGlobMeta("?") {
		t.Fatal("expected glob meta")
	}
	if pathArgHasGlobMeta("plain") || pathArgHasGlobMeta("") {
		t.Fatal("expected no glob meta")
	}
}

func TestExpandPathArgToLiteralsGlob(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.txt"), []byte("y"), 0o644)
	pat := filepath.Join(dir, "*.go")
	got, err := expandPathArgToLiterals(pat)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || filepath.Base(got[0]) != "a.go" {
		t.Fatalf("got %v", got)
	}
}

func TestTrackedMatchesScopes(t *testing.T) {
	scopes := []pushScope{
		{mountName: "m1", relPrefix: ""},
		{mountName: "m2", relPrefix: "doc"},
	}
	tf := clientdb.TrackedFile{MountName: "m2", Path: "doc/x"}
	if !trackedMatchesScopes(tf, scopes) {
		t.Fatal("expected match for m2 doc/x")
	}
	tf = clientdb.TrackedFile{MountName: "m2", Path: "other/x"}
	if trackedMatchesScopes(tf, scopes) {
		t.Fatal("expected no match")
	}
}
