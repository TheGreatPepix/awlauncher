package gamefiles

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

	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
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
		Files []BranchFile `json:"files"`
	}
	names := make([]string, 0, len(b.files))
	for name := range b.files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		data := b.files[name]
		m.Files = append(m.Files, BranchFile{Path: name, Size: int64(len(data)), SHA256: sum(data)})
	}
	out, _ := json.Marshal(m)
	return out
}

func (b *branchServer) release(name string) BranchRelease {
	return BranchRelease{
		Name:            name,
		Version:         "0.565.1",
		DownloadBaseURI: b.srv.URL + "/base",
		ManifestURL:     b.srv.URL + "/manifest",
		ManifestSHA256:  sum(b.manifest()),
		LaunchFile:      "bin64/ArmoredWarfare.exe",
	}
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
	root := BranchDir(mainRoot, "SuperTest")
	if filepath.Base(root) != "Armored Warfare SuperTest" {
		t.Fatalf("branch dir = %s", root)
	}

	state, err := SyncBranch(b.srv.Client(), b.release("SuperTest"), mainRoot, root, false)
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
	if got := readFile(t, filepath.Join(mainRoot, "bin64", platform.GameExe)); got != "main exe" {
		t.Fatalf("main install changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "gamesdk", "main-only.pak")); err == nil {
		t.Fatal("file outside the branch manifest was placed")
	}
	if state.Version != "0.565.1" || len(state.Files) != 4 {
		t.Fatalf("state = %+v", state)
	}

	if _, err := SyncBranch(b.srv.Client(), b.release("SuperTest"), mainRoot, root, false); err != nil {
		t.Fatal(err)
	}
	if got := b.downloads.Load(); got != 2 {
		t.Fatalf("unchanged branch downloaded again: %d", got)
	}

	b.files["gamesdk/textures.pak"] = bytes.Repeat([]byte("new texture "), 100000)
	delete(b.files, "system.cfg")
	if _, err := SyncBranch(b.srv.Client(), b.release("SuperTest"), mainRoot, root, false); err != nil {
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
	if _, err := SyncBranch(b.srv.Client(), b.release("SuperTest"), "", root, false); err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string][]byte{"hangar.pak": []byte("modified hangar")})
	if err := os.Remove(filepath.Join(root, "missing.pak")); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncBranch(b.srv.Client(), b.release("SuperTest"), "", root, true); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(root, "hangar.pak")); got != "modified hangar" {
		t.Fatalf("mod was replaced: %q", got)
	}
	if got := readFile(t, filepath.Join(root, "missing.pak")); got != "present" {
		t.Fatalf("missing file was not repaired: %q", got)
	}
	b.files["hangar.pak"] = []byte("new official hangar")
	if _, err := SyncBranch(b.srv.Client(), b.release("SuperTest"), "", root, true); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(root, "hangar.pak")); got != "new official hangar" {
		t.Fatalf("official update was skipped: %q", got)
	}
}

func TestDirtyBranchRechecksSameSizeContent(t *testing.T) {
	root := t.TempDir()
	b := newBranchServer(t)
	b.files = map[string][]byte{"bin64/game.exe": []byte("good")}
	state, err := SyncBranch(b.srv.Client(), b.release(DefaultBranch), "", root, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin64", "game.exe"), []byte("evil"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := MarkBranchDirty(root); err != nil {
		t.Fatal(err)
	}
	state, err = SyncBranch(b.srv.Client(), b.release(DefaultBranch), "", root, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(root, "bin64", "game.exe")); got != "good" {
		t.Fatalf("same-size content was trusted: %q", got)
	}
	if state.Dirty {
		t.Fatalf("state after repair: %+v", state)
	}
}

func TestMarkBranchDirtyPersists(t *testing.T) {
	root := t.TempDir()
	path := branchStatePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(BranchState{Branch: DefaultBranch, Files: []BranchFile{{Path: "file"}}})
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := MarkBranchDirty(root); err != nil {
		t.Fatal(err)
	}
	state, ok := ReadBranchState(root)
	if !ok || !state.Dirty {
		t.Fatalf("FX state was not marked dirty: %+v", state)
	}
}

func TestSyncBranchRestoresEditedUserCfg(t *testing.T) {
	root := t.TempDir()
	b := newBranchServer(t)
	b.files = map[string][]byte{"user.cfg": []byte("official")}
	if _, err := SyncBranch(b.srv.Client(), b.release(DefaultBranch), "", root, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "user.cfg"), []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncBranch(b.srv.Client(), b.release(DefaultBranch), "", root, false); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(root, "user.cfg")); got != "official" {
		t.Fatalf("user.cfg = %q", got)
	}
}
