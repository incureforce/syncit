package serverhttp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestHealthAndSharesStub(t *testing.T) {
	dir := t.TempDir()
	srv, err := New(Config{Addr: ":0", DataDir: filepath.Join(dir, "data")})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("health: %s", res.Status)
	}

	res, err = http.Get(ts.URL + "/v1/shares")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotImplemented {
		t.Fatalf("shares: %s", res.Status)
	}
}

func TestRegisterAndBlobUpload(t *testing.T) {
	dir := t.TempDir()
	srv, err := New(Config{Addr: ":0", DataDir: filepath.Join(dir, "data")})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/clients/register", strings.NewReader(`{"id":"c1","name":"n"}`))
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
		t.Fatalf("register: %s", res.Status)
	}

	req, err = http.NewRequest(http.MethodPost, ts.URL+"/v1/blobs", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Syncit-Client-ID", "c1")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("blob: %s %s", res.Status, string(body))
	}
}

func TestFileCatalog(t *testing.T) {
	dir := t.TempDir()
	srv, err := New(Config{Addr: ":0", DataDir: filepath.Join(dir, "data")})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/clients/register", strings.NewReader(`{"id":"c2","name":""}`))
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
		t.Fatalf("register: %s", res.Status)
	}

	req, err = http.NewRequest(http.MethodGet, ts.URL+"/v1/mounts/home/files", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Syncit-Client-ID", "c2")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("catalog: %s %s", res.Status, string(body))
	}
	if !strings.Contains(string(body), `"files"`) {
		t.Fatalf("expected files in body: %s", string(body))
	}
}

func TestMountsList(t *testing.T) {
	dir := t.TempDir()
	srv, err := New(Config{Addr: ":0", DataDir: filepath.Join(dir, "data")})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/mounts", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("mounts without client id: %s", res.Status)
	}

	req, err = http.NewRequest(http.MethodPost, ts.URL+"/v1/clients/register", strings.NewReader(`{"id":"c3","name":""}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("register: %s", res.Status)
	}

	req, err = http.NewRequest(http.MethodGet, ts.URL+"/v1/mounts", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Syncit-Client-ID", "c3")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("mounts list: %s %s", res.Status, string(body))
	}
	var out struct {
		Mounts []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"mounts"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if out.Mounts == nil {
		t.Fatal("expected mounts array (possibly empty)")
	}
	if len(out.Mounts) != 0 {
		t.Fatalf("expected no mounts yet, got %d", len(out.Mounts))
	}
}

func TestPutClientMountsLinksList(t *testing.T) {
	dir := t.TempDir()
	srv, err := New(Config{Addr: ":0", DataDir: filepath.Join(dir, "data")})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/clients/register", strings.NewReader(`{"id":"c4","name":""}`))
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
		t.Fatalf("register: %s", res.Status)
	}

	req, err = http.NewRequest(http.MethodPut, ts.URL+"/v1/client/mounts", strings.NewReader(`{"mounts":["alpha","beta"]}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Syncit-Client-ID", "c4")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("put client mounts: %s %s", res.Status, string(body))
	}

	req, err = http.NewRequest(http.MethodGet, ts.URL+"/v1/mounts", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Syncit-Client-ID", "c4")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("mounts list: %s %s", res.Status, string(body))
	}
	var list struct {
		Mounts []struct {
			Name string `json:"name"`
		} `json:"mounts"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Mounts) != 2 {
		t.Fatalf("expected 2 mounts, got %+v", list.Mounts)
	}
}

func TestPutClientMountsReplaces(t *testing.T) {
	dir := t.TempDir()
	srv, err := New(Config{Addr: ":0", DataDir: filepath.Join(dir, "data")})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/clients/register", strings.NewReader(`{"id":"c5","name":""}`))
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
		t.Fatalf("register: %s", res.Status)
	}

	put := func(body string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPut, ts.URL+"/v1/client/mounts", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Syncit-Client-ID", "c5")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("put client mounts: %s %s", res.Status, string(b))
		}
	}
	put(`{"mounts":["m1","m2"]}`)
	put(`{"mounts":["m1"]}`)

	req, err = http.NewRequest(http.MethodGet, ts.URL+"/v1/mounts", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Syncit-Client-ID", "c5")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("mounts list: %s %s", res.Status, string(body))
	}
	var list struct {
		Mounts []struct {
			Name string `json:"name"`
		} `json:"mounts"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatal(err)
	}
	// Global catalog still lists m2; client c5 only links m1 via PUT.
	if len(list.Mounts) != 2 {
		t.Fatalf("expected global mounts m1 and m2, got %+v", list.Mounts)
	}
}
