package serverdb

import (
	"context"
	"path/filepath"
	"sort"
	"testing"
)

func TestClientIDsAffectedByFilePush(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := OpenServer(filepath.Join(dir, "srv.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	must(db.RegisterClient(ctx, "pusher", ""))
	must(db.RegisterClient(ctx, "peer_ok", ""))
	must(db.RegisterClient(ctx, "peer_bad_tags", ""))

	must(db.SetClientTags(ctx, "pusher", []string{"t1"}))
	must(db.SetClientTags(ctx, "peer_ok", []string{"t1"}))
	must(db.SetClientTags(ctx, "peer_bad_tags", []string{"t2"}))

	must(db.SyncClientMounts(ctx, "pusher", []string{"m"}))
	must(db.SyncClientMounts(ctx, "peer_ok", []string{"m"}))
	must(db.SyncClientMounts(ctx, "peer_bad_tags", []string{"m"}))

	ids, err := db.ClientIDsAffectedByFilePush(ctx, "m", []string{"t1"}, "pusher")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(ids)
	if len(ids) != 1 || ids[0] != "peer_ok" {
		t.Fatalf("want [peer_ok], got %q", ids)
	}

	ids2, err := db.ClientIDsAffectedByFilePush(ctx, "m", nil, "pusher")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(ids2)
	if len(ids2) != 2 {
		t.Fatalf("empty file tags should notify all linked peers except pusher: got %q", ids2)
	}
}
