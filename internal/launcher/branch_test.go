package launcher

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
)

func sum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeTree(t *testing.T, root string, files map[string][]byte) {
	t.Helper()
	for name, data := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0755)
		if err := os.WriteFile(p, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

type branchServer struct {
	srv       *httptest.Server
	files     map[string][]byte
	downloads atomic.Int32
}

func newBranchServer(t *testing.T) *branchServer {
	b := &branchServer{files: map[string][]byte{}}
	b.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manifest" {
			w.Write(b.manifest())
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/base/data/")
		data, ok := b.files[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		b.downloads.Add(1)
		w.Write(data)
	}))
	t.Cleanup(b.srv.Close)
	return b
}

func (b *branchServer) manifest() []byte {
	var m struct {
		Files []fxFile `json:"files"`
	}
	names := make([]string, 0, len(b.files))
	for name := range b.files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		data := b.files[name]
		m.Files = append(m.Files, fxFile{Path: name, Size: int64(len(data)), SHA256: sum(data)})
	}
	out, _ := json.Marshal(m)
	return out
}

func (b *branchServer) branchManifest() fxBranchManifest {
	var m fxBranchManifest
	lc := &m.Manifest.LauncherConfiguration
	lc.HTTPDownload.DownloadBaseURI = b.srv.URL + "/base"
	lc.ManifestURL = b.srv.URL + "/manifest"
	lc.ManifestSHA256 = sum(b.manifest())
	lc.LaunchFile = "bin64/ArmoredWarfare.exe"
	m.Manifest.Release.BuildVersion = "0.565.1"
	return m
}

func TestSyncBranchLinksSharedFiles(t *testing.T) {
	big := bytes.Repeat([]byte("shared texture "), 100000)
	mainRoot := filepath.Join(t.TempDir(), "Armored Warfare")
	writeTree(t, mainRoot, map[string][]byte{
		"gamesdk/textures.pak":     big,
		"system.cfg":               []byte("shared small config"),
		"bin64/ArmoredWarfare.exe": []byte("main exe"),
		"gamesdk/main-only.pak":    []byte("not in the branch"),
	})
	b := newBranchServer(t)
	b.files = map[string][]byte{
		"gamesdk/textures.pak":     big,
		"system.cfg":               []byte("shared small config"),
		"bin64/ArmoredWarfare.exe": []byte("supertest exe"),
		"user.cfg":                 []byte("net_frontline_address = beta"),
	}
	root := branchInstallDir(mainRoot, "SuperTest")
	if filepath.Base(root) != "Armored Warfare SuperTest" {
		t.Fatalf("branch dir = %s", root)
	}

	state, err := syncBranch(b.srv.Client(), "SuperTest", b.branchManifest(), mainRoot, root)
	if err != nil {
		t.Fatal(err)
	}
	if got := b.downloads.Load(); got != 2 {
		t.Fatalf("downloaded %d files, want 2 (exe and user.cfg)", got)
	}
	same := func(name string) bool {
		a, _ := os.Stat(filepath.Join(mainRoot, filepath.FromSlash(name)))
		c, _ := os.Stat(filepath.Join(root, filepath.FromSlash(name)))
		return a != nil && c != nil && os.SameFile(a, c)
	}
	if !same("gamesdk/textures.pak") {
		t.Fatal("large shared file was not hard-linked")
	}
	if same("system.cfg") {
		t.Fatal("small config must be copied, not linked")
	}
	if got := readFile(t, filepath.Join(mainRoot, "bin64", "ArmoredWarfare.exe")); got != "main exe" {
		t.Fatalf("main install changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "gamesdk", "main-only.pak")); err == nil {
		t.Fatal("file outside the branch manifest was placed")
	}
	if state.Version != "0.565.1" || len(state.Files) != 4 {
		t.Fatalf("state = %+v", state)
	}

	if _, err := syncBranch(b.srv.Client(), "SuperTest", b.branchManifest(), mainRoot, root); err != nil {
		t.Fatal(err)
	}
	if got := b.downloads.Load(); got != 2 {
		t.Fatalf("unchanged branch downloaded again: %d", got)
	}

	b.files["gamesdk/textures.pak"] = bytes.Repeat([]byte("new texture "), 100000)
	delete(b.files, "system.cfg")
	if _, err := syncBranch(b.srv.Client(), "SuperTest", b.branchManifest(), mainRoot, root); err != nil {
		t.Fatal(err)
	}
	if got := b.downloads.Load(); got != 3 {
		t.Fatalf("update downloaded %d files in total, want 3", got)
	}
	if !bytes.Equal([]byte(readFile(t, filepath.Join(mainRoot, "gamesdk", "textures.pak"))), big) {
		t.Fatal("updating the branch changed the main install")
	}
	if _, err := os.Stat(filepath.Join(root, "system.cfg")); err == nil {
		t.Fatal("file removed from the branch was kept")
	}
}

func TestSyncBranchPreservesModsUntilOfficialFileChanges(t *testing.T) {
	root := t.TempDir()
	b := newBranchServer(t)
	b.files = map[string][]byte{"hangar.pak": []byte("original"), "missing.pak": []byte("present")}
	if _, err := syncBranch(b.srv.Client(), "SuperTest", b.branchManifest(), "", root); err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string][]byte{"hangar.pak": []byte("modified hangar")})
	if err := os.Remove(filepath.Join(root, "missing.pak")); err != nil {
		t.Fatal(err)
	}
	if _, err := syncBranch(b.srv.Client(), "SuperTest", b.branchManifest(), "", root, true); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(root, "hangar.pak")); got != "modified hangar" {
		t.Fatalf("mod was replaced: %q", got)
	}
	if got := readFile(t, filepath.Join(root, "missing.pak")); got != "present" {
		t.Fatalf("missing file was not repaired: %q", got)
	}
	b.files["hangar.pak"] = []byte("new official hangar")
	if _, err := syncBranch(b.srv.Client(), "SuperTest", b.branchManifest(), "", root, true); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(root, "hangar.pak")); got != "new official hangar" {
		t.Fatalf("official update was skipped: %q", got)
	}
}
