package serverhttp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSSEFanoutOnPush(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	srv, err := New(Config{Addr: ":0", DataDir: filepath.Join(dir, "data")})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	reg := func(id string) {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/v1/clients/register",
			strings.NewReader(fmt.Sprintf(`{"id":%q,"name":""}`, id)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("register %s: %s", id, res.Status)
		}
	}
	reg("u1")
	reg("u2")

	putMounts := func(clientID, body string) {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, ts.URL+"/v1/client/mounts", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(syncitClientHeader, clientID)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("mounts %s: %s", clientID, res.Status)
		}
	}
	putMounts("u1", `{"mounts":["ssemt"]}`)
	putMounts("u2", `{"mounts":["ssemt"]}`)

	syncBody, err := json.Marshal(map[string]string{
		"synced_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, ts.URL+"/v1/client/sync-state", strings.NewReader(string(syncBody)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(syncitClientHeader, "u2")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("sync-state: %s", res.Status)
	}

	evCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err = http.NewRequestWithContext(evCtx, http.MethodGet, ts.URL+"/v1/client/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(syncitClientHeader, "u2")

	go func() {
		time.Sleep(200 * time.Millisecond)
		reqb, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/v1/blobs", strings.NewReader("hello"))
		if err != nil {
			t.Error(err)
			return
		}
		reqb.Header.Set(syncitClientHeader, "u1")
		res, err := http.DefaultClient.Do(reqb)
		if err != nil {
			t.Error(err)
			return
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Errorf("blob: %s %s", res.Status, string(body))
			return
		}
		var out struct {
			Hash string `json:"hash"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			t.Error(err)
			return
		}
		pushBody := fmt.Sprintf(`{"path":"a.txt","tags":[],"file_hash":%q,"file_size":5}`, out.Hash)
		reqp, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/v1/mounts/ssemt/files", strings.NewReader(pushBody))
		if err != nil {
			t.Error(err)
			return
		}
		reqp.Header.Set("Content-Type", "application/json")
		reqp.Header.Set(syncitClientHeader, "u1")
		resp, err := http.DefaultClient.Do(reqp)
		if err != nil {
			t.Error(err)
			return
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("push: %s", resp.Status)
		}
	}()

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("events: %s", res.Status)
	}
	br := bufio.NewReader(res.Body)
	deadline := time.After(5 * time.Second)
	var saw bool
	for !saw {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for SSE data")
		default:
		}
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(line, "data:") {
			if strings.Contains(line, "file_version") && strings.Contains(line, "a.txt") {
				saw = true
				cancel()
			}
		}
	}
	if !saw {
		t.Fatal("expected file_version event")
	}
}
