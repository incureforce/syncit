package mount

import (
	"path/filepath"
	"strings"
)

// Entry is one client mount (name + absolute root).
type Entry struct {
	Name     string
	RootPath string
}

// ResolveNearest picks the mount whose root is the longest prefix of absPath.
// absPath must be absolute and clean. Returns relative path under that mount.
func ResolveNearest(absPath string, mounts []Entry) (name string, root string, rel string, ok bool) {
	absPath = filepath.Clean(absPath)
	var best *Entry
	bestLen := -1
	for i := range mounts {
		m := &mounts[i]
		root := filepath.Clean(m.RootPath)
		r, err := filepath.Rel(root, absPath)
		if err != nil {
			continue
		}
		if strings.HasPrefix(r, "..") {
			continue
		}
		if len(root) > bestLen {
			bestLen = len(root)
			best = &mounts[i]
		}
	}
	if best == nil {
		return "", "", "", false
	}
	root = filepath.Clean(best.RootPath)
	r, err := filepath.Rel(root, absPath)
	if err != nil {
		return "", "", "", false
	}
	r = filepath.ToSlash(r)
	if r == "." {
		r = ""
	}
	return best.Name, root, r, true
}
