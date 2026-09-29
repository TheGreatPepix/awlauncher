package gamefiles

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/torrent"
)

func md5Hex(s string) string { sum := md5.Sum([]byte(s)); return hex.EncodeToString(sum[:]) }

func fileServer(t *testing.T, files map[string]string, requests *atomic.Int32) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := files[r.URL.Path]
		if !ok {
			t.Errorf("unexpected download %q", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if requests != nil {
			requests.Add(1)
		}
		_, _ = w.Write([]byte(data))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSyncFilesDownloadsOnlyMissingAndCorrupt(t *testing.T) {
	var requests atomic.Int32
	srv := fileServer(t, map[string]string{"/bad.dat": "good", "/missing.dat": "ok"}, &requests)
	root := t.TempDir()
	writeTree(t, root, map[string][]byte{"good.dat": []byte("good"), "bad.dat": []byte("evil")})
	set := fileSet{NewHash: md5.New, Algo: "MD5", Files: []remoteFile{
		{Path: "good.dat", Size: 4, Hash: md5Hex("good"), URL: srv.URL + "/good.dat"},
		{Path: "bad.dat", Size: 4, Hash: md5Hex("good"), URL: srv.URL + "/bad.dat"},
		{Path: "missing.dat", Size: 2, Hash: md5Hex("ok"), URL: srv.URL + "/missing.dat"},
	}}
	fetched, err := syncFiles(root, set, syncOptions{Jobs: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(fetched) != 2 || requests.Load() != 2 {
		t.Fatalf("fetched %v with %d requests", fetched, requests.Load())
	}
	for name, want := range map[string]string{"good.dat": "good", "bad.dat": "good", "missing.dat": "ok"} {
		if got := readFile(t, filepath.Join(root, name)); got != want {
			t.Fatalf("%s = %q", name, got)
		}
	}
}

func TestSyncFilesDeclineKeepsFiles(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string][]byte{"bad.dat": []byte("evil")})
	set := fileSet{NewHash: md5.New, Algo: "MD5", Files: []remoteFile{{Path: "bad.dat", Size: 4, Hash: md5Hex("good"), URL: "http://unused/"}}}
	_, err := syncFiles(root, set, syncOptions{Confirm: func([]remoteFile, int64) error { return ErrDeclined }})
	if !errors.Is(err, ErrDeclined) {
		t.Fatalf("err = %v", err)
	}
	if got := readFile(t, filepath.Join(root, "bad.dat")); got != "evil" {
		t.Fatalf("declined repair changed the file: %q", got)
	}
}

func TestSyncFilesRejectsCorruptDownload(t *testing.T) {
	srv := fileServer(t, map[string]string{"/file.dll": "bad!"}, nil)
	root := t.TempDir()
	set := fileSet{NewHash: md5.New, Algo: "MD5", Files: []remoteFile{{Path: "file.dll", Size: 4, Hash: md5Hex("good"), URL: srv.URL + "/file.dll"}}}
	if _, err := syncFiles(root, set, syncOptions{}); err == nil {
		t.Fatal("expected MD5 verification to reject corrupt download")
	}
	if _, err := os.Stat(filepath.Join(root, "file.dll")); !os.IsNotExist(err) {
		t.Fatalf("corrupt file remains: %v", err)
	}
}

func TestVKDistribFilesSelectsDamagedWithWebseedURL(t *testing.T) {
	d := vkDistrib{
		meta: torrent.Meta{Name: "client", Webseed: "http://seed/", Files: []torrent.File{
			{Name: "bin64/missing.dll", Size: 3},
			{Name: "bin64/present.dll", Size: 7},
		}},
		manifest: Manifest{NonCompressed: FileGroup{Files: []ManifestFile{
			{Name: `bin64\missing.dll`, Size: 3, MD5: md5Hex("abc")},
			{Name: `bin64\present.dll`, Size: 7, MD5: md5Hex("present")},
		}}},
	}
	set, err := d.files([]string{`BIN64\missing.dll`})
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Files) != 1 || set.Files[0].URL != "http://seed/client/bin64/missing.dll" || set.Files[0].Hash != md5Hex("abc") {
		t.Fatalf("files = %+v", set.Files)
	}
	d.manifest.NonCompressed.Files[0].Size = 4
	if _, err := d.files([]string{`bin64\missing.dll`}); err == nil {
		t.Fatal("expected a manifest size mismatch")
	}
}
