package mount

import (
	"path/filepath"
	"testing"
)

func TestResolveNearest(t *testing.T) {
	m := []Entry{
		{Name: "home", RootPath: "/Users/u"},
		{Name: "proj", RootPath: "/Users/u/project"},
	}
	gotName, _, rel, ok := ResolveNearest("/Users/u/project/src/foo.txt", m)
	if !ok {
		t.Fatal("expected match")
	}
	if gotName != "proj" {
		t.Fatalf("name: got %q want proj", gotName)
	}
	if rel != "src/foo.txt" {
		t.Fatalf("rel: got %q", rel)
	}

	_, _, _, ok = ResolveNearest("/other/outside", m)
	if ok {
		t.Fatal("expected no match")
	}
}

func TestResolveNearestExactRoot(t *testing.T) {
	m := []Entry{{Name: "a", RootPath: "/a"}}
	name, _, rel, ok := ResolveNearest("/a", m)
	if !ok || name != "a" || rel != "" {
		t.Fatalf("got ok=%v name=%q rel=%q", ok, name, rel)
	}
}

func TestResolveNearestWindowsStyle(t *testing.T) {
	if filepath.Separator != '\\' {
		t.Skip("windows only")
	}
	m := []Entry{
		{Name: "c", RootPath: `C:\Users`},
		{Name: "d", RootPath: `C:\Users\bob\dev`},
	}
	name, _, rel, ok := ResolveNearest(`C:\Users\bob\dev\src\main.go`, m)
	if !ok {
		t.Fatal("expected match")
	}
	if name != "d" {
		t.Fatalf("name %q", name)
	}
	if rel != "src/main.go" && rel != `src\main.go` {
		t.Fatalf("rel %q", rel)
	}
}
