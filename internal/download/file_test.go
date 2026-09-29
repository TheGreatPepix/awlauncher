package download

import (
	"bytes"
	"context"
	"crypto/md5"
	"errors"
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
	sum, err := Fetch(context.Background(), Request{Client: &http.Client{}, URL: srv.URL, Dst: dst, Label: "file.dat", Size: int64(len(content)), NewHash: md5.New})
	if err != nil {
		t.Fatal(err)
	}
	if want := md5.Sum([]byte(content)); !bytes.Equal(sum, want[:]) {
		t.Fatalf("sum %x, want %x", sum, want)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("a stalled connection held the download for %s", elapsed)
	}
	if data, _ := os.ReadFile(dst); string(data) != content {
		t.Fatalf("downloaded %q", data)
	}
}

func TestFetchHashesWhatItDownloads(t *testing.T) {
	content := "the whole file, sent in one piece or resumed"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "file.dat", time.Time{}, strings.NewReader(content))
	}))
	defer srv.Close()
	want := md5.Sum([]byte(content))
	for _, partial := range []string{"", content[:7], content} {
		dst := filepath.Join(t.TempDir(), "file.dat")
		if partial != "" {
			os.WriteFile(dst+".part", []byte(partial), 0644)
		}
		sum, err := Fetch(context.Background(), Request{Client: srv.Client(), URL: srv.URL, Dst: dst, Label: "file.dat", Size: int64(len(content)), NewHash: md5.New})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(sum, want[:]) {
			t.Fatalf("with %d bytes already there: sum %x, want %x", len(partial), sum, want)
		}
		if data, _ := os.ReadFile(dst); string(data) != content {
			t.Fatalf("downloaded %q", data)
		}
	}
}

func TestFetchStopsWhenCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		fmt.Fprint(w, "part")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	start := time.Now()
	_, err := Fetch(ctx, Request{Client: srv.Client(), URL: srv.URL, Dst: filepath.Join(t.TempDir(), "file.dat"), Label: "file.dat", Size: 100})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("cancelling took %s", elapsed)
	}
}
