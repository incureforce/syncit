package cli

import (
	"context"

	"go-syncit/internal/client/db"
	"go-syncit/internal/mount"
)

func resolveToTracked(db *clientdb.ClientDB, ctx context.Context, pathArg string) (clientdb.TrackedFile, error) {
	abs, err := expandPath(pathArg)
	if err != nil {
		return clientdb.TrackedFile{}, err
	}
	mounts, err := db.ListMounts(ctx)
	if err != nil {
		return clientdb.TrackedFile{}, err
	}
	entries := make([]mount.Entry, 0, len(mounts))
	for _, m := range mounts {
		entries = append(entries, mount.Entry{Name: m.Name, RootPath: m.RootPath})
	}
	name, _, rel, ok := mount.ResolveNearest(abs, entries)
	if !ok {
		return clientdb.TrackedFile{}, errPathOutsideMounts
	}
	return db.TrackedByMountAndPath(ctx, name, rel)
}
