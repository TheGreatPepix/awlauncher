package download

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDownloadOneResumes(t *testing.T) {
	content := "a complete patch payload"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=2-" {
			t.Errorf("unexpected range %q", r.Header.Get("Range"))
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 2-%d/%d", len(content)-1, len(content)))
		w.WriteHeader(http.StatusPartialContent)
		fmt.Fprint(w, content[2:])
	}))
	defer srv.Close()
	dst := filepath.Join(t.TempDir(), "patch.7z.001")
	if err := os.WriteFile(dst+".part", []byte(content[:2]), 0644); err != nil {
		t.Fatal(err)
	}
	if err := File(srv.Client(), srv.URL, dst, "patch.7z.001", int64(len(content))); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != content {
		t.Fatalf("downloaded %q", data)
	}
}

func TestDownloadResumesAfterAStalledConnection(t *testing.T) {
	old := stallTimeout
	stallTimeout = 200 * time.Millisecond
	t.Cleanup(func() { stallTimeout = old })
	content := "0123456789abcdefghij"
	release := make(chan struct{})
	defer close(release)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "" {
			w.Header().Set("Content-Length", fmt.Sprint(len(content)))
			fmt.Fprint(w, content[:10])
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-release:
			}
			return
		}
		if r.Header.Get("Range") != "bytes=10-" {
			t.Errorf("unexpected range %q", r.Header.Get("Range"))
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 10-%d/%d", len(content)-1, len(content)))
		w.WriteHeader(http.StatusPartialContent)
		fmt.Fprint(w, content[10:])
	}))
	defer srv.Close()
	dst := filepath.Join(t.TempDir(), "file.dat")
	start := time.Now()
	if err := File(&http.Client{}, srv.URL, dst, "file.dat", int64(len(content))); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("a stalled connection held the download for %s", elapsed)
	}
	if data, _ := os.ReadFile(dst); string(data) != content {
		t.Fatalf("downloaded %q", data)
	}
}
