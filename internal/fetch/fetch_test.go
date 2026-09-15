package fetch

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.grayc.dev/grayc-devops/helm-manifest-renderer/internal/config"
)

func testSource() config.URLSource {
	return config.URLSource{
		Repo:    "kubernetes-sigs/cluster-api",
		Version: "v1.12.7",
		Asset:   "cluster-api-components.yaml",
	}
}

func TestAssetURL(t *testing.T) {
	got := AssetURL("https://github.com", testSource())
	want := "https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.12.7/cluster-api-components.yaml"
	if got != want {
		t.Errorf("AssetURL()\n got: %s\nwant: %s", got, want)
	}
}

func TestAssetURLTrimsTrailingSlash(t *testing.T) {
	got := AssetURL("https://github.com/", testSource())
	if strings.Contains(got, "com//") {
		t.Errorf("AssetURL() double slash: %s", got)
	}
}

func TestFetchFromWritesTheAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/kubernetes-sigs/cluster-api/releases/download/v1.12.7/cluster-api-components.yaml" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte("kind: Namespace\n"))
	}))
	defer server.Close()

	dest := t.TempDir()
	if err := FetchFrom(server.URL, testSource(), dest); err != nil {
		t.Fatalf("FetchFrom() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dest, "cluster-api-components.yaml"))
	if err != nil {
		t.Fatalf("reading fetched asset: %v", err)
	}
	if string(content) != "kind: Namespace\n" {
		t.Errorf("content = %q", string(content))
	}
}

func TestFetchFromFailsOnNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer server.Close()

	err := FetchFrom(server.URL, testSource(), t.TempDir())
	if err == nil {
		t.Fatal("FetchFrom() error = nil, want a 404 error")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error does not name the status code: %v", err)
	}
}

func TestFetchFromFailsOnServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	err := FetchFrom(server.URL, testSource(), t.TempDir())
	if err == nil {
		t.Fatal("FetchFrom() error = nil, want a 500 error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error does not name the status code: %v", err)
	}
}

func TestFetchFromWritesNoFileOnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer server.Close()

	dest := t.TempDir()
	_ = FetchFrom(server.URL, testSource(), dest)

	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatalf("reading dest: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("a failed fetch left %d file(s) behind", len(entries))
	}
}

func TestFetchFromFailsOnTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte("kind: Namespace\n"))
	}))
	defer server.Close()

	old := Client
	Client = &http.Client{Timeout: 50 * time.Millisecond}
	defer func() { Client = old }()

	dest := t.TempDir()
	err := FetchFrom(server.URL, testSource(), dest)
	if err == nil {
		t.Fatal("FetchFrom() error = nil, want a timeout error")
	}

	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatalf("reading dest: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("a timed-out fetch left %d file(s) behind", len(entries))
	}
}

func TestFetchFromLeavesNoPartialFileOnCopyFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.Write([]byte("partial"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		panic(http.ErrAbortHandler)
	}))
	defer server.Close()

	dest := t.TempDir()
	err := FetchFrom(server.URL, testSource(), dest)
	if err == nil {
		t.Fatal("FetchFrom() error = nil, want a copy failure error")
	}
	if !strings.Contains(err.Error(), "write ") {
		t.Errorf("error should be a write error, got: %v", err)
	}

	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatalf("reading dest: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("a failed copy left %d file(s) behind", len(entries))
	}
}
