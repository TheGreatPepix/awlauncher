package torrent

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

func TestBadFilesFindsCorruptFile(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{"a.bin": "0123456789", "b.bin": "abcdefghij", "c.bin": "ABCDEFGHIJ"}
	meta := Meta{PieceSize: 8}
	var stream string
	for _, name := range []string{"a.bin", "b.bin", "c.bin"} {
		meta.Files = append(meta.Files, File{Name: name, Size: int64(len(files[name]))})
		stream += files[name]
		if err := os.WriteFile(filepath.Join(root, name), []byte(files[name]), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < len(stream); i += 8 {
		end := min(i+8, len(stream))
		sum := sha1.Sum([]byte(stream[i:end]))
		meta.Hashes = append(meta.Hashes, sum[:]...)
	}
	if bad, err := meta.badFiles(root); err != nil || len(bad) != 0 {
		t.Fatalf("clean: %v, %v", bad, err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.bin"), []byte("abXdefghij"), 0644); err != nil {
		t.Fatal(err)
	}
	bad, err := meta.badFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.bin", "b.bin"}; !reflect.DeepEqual(bad, want) {
		t.Fatalf("bad = %v, want %v", bad, want)
	}
	if meta.verifyPieces(root) == nil {
		t.Fatal("verifyPieces accepted a corrupt file")
	}
}

func TestBadFilesOverManyPieces(t *testing.T) {
	root := t.TempDir()
	meta := Meta{PieceSize: 64}
	var stream []byte
	var names []string
	for i := range 60 {
		if i == 30 {
			meta.Files = append(meta.Files, File{Name: "pad", Size: 37, Padding: true})
			stream = append(stream, make([]byte, 37)...)
		}
		name := fmt.Sprintf("f%02d.bin", i)
		data := make([]byte, 1+(i*53)%211)
		for j := range data {
			data[j] = byte(i*7 + j)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0644); err != nil {
			t.Fatal(err)
		}
		meta.Files = append(meta.Files, File{Name: name, Size: int64(len(data))})
		names = append(names, name)
		stream = append(stream, data...)
	}
	for i := 0; i < len(stream); i += 64 {
		sum := sha1.Sum(stream[i:min(i+64, len(stream))])
		meta.Hashes = append(meta.Hashes, sum[:]...)
	}
	for _, name := range []string{"f03.bin", "f31.bin", "f59.bin"} {
		path := filepath.Join(root, name)
		data, _ := os.ReadFile(path)
		data[len(data)/2] ^= 0xff
		os.WriteFile(path, data, 0644)
	}
	var want []string
	var disk []byte
	for _, f := range meta.Files {
		if f.Padding {
			disk = append(disk, make([]byte, f.Size)...)
			continue
		}
		data, _ := os.ReadFile(filepath.Join(root, f.Name))
		disk = append(disk, data...)
	}
	seen := map[string]bool{}
	for p := 0; p*64 < len(disk); p++ {
		start, end := p*64, min(p*64+64, len(disk))
		if sum := sha1.Sum(disk[start:end]); reflect.DeepEqual(sum[:], meta.Hashes[p*20:p*20+20]) {
			continue
		}
		var off int
		for _, f := range meta.Files {
			if !f.Padding && off < end && start < off+int(f.Size) && !seen[f.Name] {
				seen[f.Name] = true
				want = append(want, f.Name)
			}
			off += int(f.Size)
		}
	}
	bad, err := meta.badFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(bad, want) || len(want) == 0 {
		t.Fatalf("bad = %v, want %v", bad, want)
	}

	os.WriteFile(filepath.Join(root, names[10]), []byte("short"), 0644)
	if _, err := meta.badFiles(root); err == nil {
		t.Fatal("a truncated file passed verification")
	}
}

func TestPrefetchDownloadsQuietlyAndStopsOnCancel(t *testing.T) {
	files := map[string]string{"/pack/a.bin": "first file", "/pack/b.bin": "second file"}
	block := make(chan struct{})
	defer close(block)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pack/slow.bin" {
			w.Header().Set("Content-Length", "10")
			w.Write([]byte("sl"))
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-block:
			}
			return
		}
		http.ServeContent(w, r, "", time.Time{}, strings.NewReader(files[r.URL.Path]))
	}))
	defer srv.Close()
	root := t.TempDir()
	meta := Meta{Name: "pack", Webseed: srv.URL + "/", Files: []File{{Name: "a.bin", Size: 10}, {Name: "b.bin", Size: 11}}}
	if meta.Missing(root) != 21 {
		t.Fatalf("missing %d before the download", meta.Missing(root))
	}
	board := progress.Default.Quiet()
	if err := meta.Prefetch(context.Background(), srv.Client(), root, 2, board); err != nil {
		t.Fatal(err)
	}
	if meta.Missing(root) != 0 {
		t.Fatalf("missing %d after the download", meta.Missing(root))
	}
	if progress.Default.Snapshot().Active {
		t.Fatal("prefetch showed up on the progress board")
	}

	slow := Meta{Name: "pack", Webseed: srv.URL + "/", Files: []File{{Name: "slow.bin", Size: 10}}}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	start := time.Now()
	if err := slow.Prefetch(ctx, srv.Client(), root, 1, board); !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("cancelling took %s", elapsed)
	}
}

func TestDownloadReplacesStalePreloadedFile(t *testing.T) {
	const content = "the published patch payload"
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/patch/payload.bin" {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, "payload.bin", time.Time{}, strings.NewReader(content))
	}))
	defer srv.Close()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "payload.bin"), []byte(strings.Repeat("x", len(content))), 0644); err != nil {
		t.Fatal(err)
	}
	sum := sha1.Sum([]byte(content))
	meta := Meta{Name: "patch", Webseed: srv.URL + "/", PieceSize: 64, Hashes: sum[:], Files: []File{{Name: "payload.bin", Size: int64(len(content))}}}
	if err := meta.Download(srv.Client(), root, 1); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "payload.bin"))
	if err != nil || string(data) != content {
		t.Fatalf("repaired file = %q, %v", data, err)
	}
	if requests.Load() != 1 {
		t.Fatalf("download requests = %d, want 1", requests.Load())
	}
	if err := meta.Download(srv.Client(), root, 1); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 {
		t.Fatalf("cached file was downloaded again: %d requests", requests.Load())
	}
}
