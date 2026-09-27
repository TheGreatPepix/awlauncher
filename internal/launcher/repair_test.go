package launcher

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRepairListedFilesDownloadsOnlyDamagedFile(t *testing.T) {
	content := []byte("repaired client file")
	sum := md5.Sum(content)
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path != "/client/bin64/missing.dll" {
			t.Errorf("unexpected download %q", r.URL.Path)
		}
		_, _ = w.Write(content)
	}))
	defer srv.Close()
	root := t.TempDir()
	meta := torrentMeta{name: "client", webseed: srv.URL + "/", files: []torrentFile{
		{name: "bin64/missing.dll", size: int64(len(content))},
		{name: "bin64/present.dll", size: 7},
	}}
	manifest := patchManifest{NonCompressed: fileGroup{Files: []manifestFile{
		{Name: `bin64\missing.dll`, Size: int64(len(content)), MD5: hex.EncodeToString(sum[:])},
	}}}
	damaged := []inventoryFile{{Name: `bin64\missing.dll`, Size: int64(len(content))}}
	if err := repairListedFiles(srv.Client(), root, meta, manifest, damaged); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "bin64", "missing.dll"))
	if err != nil || string(got) != string(content) || requestCount != 1 {
		t.Fatalf("downloaded %q, requests=%d, err=%v", got, requestCount, err)
	}
}

func TestRepairListedFilesRejectsManifestMismatch(t *testing.T) {
	root := t.TempDir()
	meta := torrentMeta{files: []torrentFile{{name: "bin64/file.dll", size: 5}}}
	manifest := patchManifest{NonCompressed: fileGroup{Files: []manifestFile{{Name: `bin64\file.dll`, Size: 4}}}}
	err := repairListedFiles(http.DefaultClient, root, meta, manifest, []inventoryFile{{Name: `bin64\file.dll`, Size: 5}})
	if err == nil {
		t.Fatal("expected a manifest size mismatch")
	}
}

func TestRepairListedFilesRejectsCorruptDownload(t *testing.T) {
	good := []byte("good")
	sum := md5.Sum(good)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("bad!"))
	}))
	defer srv.Close()
	root := t.TempDir()
	meta := torrentMeta{name: "client", webseed: srv.URL + "/", files: []torrentFile{{name: "file.dll", size: 4}}}
	manifest := patchManifest{NonCompressed: fileGroup{Files: []manifestFile{{Name: "file.dll", Size: 4, MD5: hex.EncodeToString(sum[:])}}}}
	err := repairListedFiles(srv.Client(), root, meta, manifest, []inventoryFile{{Name: "file.dll", Size: 4}})
	if err == nil {
		t.Fatal("expected MD5 verification to reject corrupt download")
	}
	if _, err := os.Stat(filepath.Join(root, "file.dll")); !os.IsNotExist(err) {
		t.Fatalf("corrupt file remains: %v", err)
	}
}
