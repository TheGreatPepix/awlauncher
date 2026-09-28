package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
)

func writeBranchState(t *testing.T, dir string, s gamefiles.BranchState) {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, dir, map[string][]byte{"-gup-/awlauncher/branch.json": data})
}
func gameSession(t *testing.T, root string, answer bool) *Session {
	t.Helper()
	t.Setenv("LOCALAPPDATA", t.TempDir())
	return NewSession(config.NewStore(config.Config{Game: root})).WithUI(&fakeUI{yes: answer})
}
func TestConfiguredClientFolders(t *testing.T) {
	parent := t.TempDir()
	vk := filepath.Join(parent, "VK")
	fx := filepath.Join(parent, "FX")
	branch := filepath.Join(parent, "Other drive", "SuperTest")
	writeTree(t, vk, map[string][]byte{"-gup-/last.xml": []byte(`<Manifest Build="442"/>`)})
	writeBranchState(t, vk, gamefiles.BranchState{Branch: gamefiles.DefaultBranch, Version: "stale"})
	writeBranchState(t, fx, gamefiles.BranchState{Branch: gamefiles.DefaultBranch, Version: "main"})
	writeBranchState(t, branch, gamefiles.BranchState{Branch: "SuperTest", Version: "branch"})
	cfg := config.Config{Game: vk, FXGame: fx, BranchGames: map[string]string{"supertest": branch}}
	if got := cfg.BranchDir("SuperTest"); got != branch {
		t.Fatalf("custom branch folder = %q", got)
	}
	clients := installedConfiguredClients(cfg)
	if len(clients) != 3 || clients[0].Kind != gamefiles.KindVK || clients[0].Dir != vk || clients[1].Kind != gamefiles.KindFX || clients[1].Dir != fx || clients[2].Dir != branch {
		t.Fatalf("clients = %+v", clients)
	}
	cfg.BranchGames = nil
	if got := cfg.BranchDir("SuperTest"); got != gamefiles.BranchDir(fx, "SuperTest") {
		t.Fatalf("default branch folder = %q", got)
	}
}
func TestDescribeGameWithoutInstalledClientsHasArray(t *testing.T) {
	info := describeGame(t.TempDir())
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	var event map[string]json.RawMessage
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatal(err)
	}
	if string(event["clients"]) != "[]" {
		t.Fatalf("clients must be an array for the UI, got %s", event["clients"])
	}
}
func TestRemoveClientDirKeepsFilesItDoesNotKnow(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Armored Warfare SuperTest")
	writeTree(t, dir, map[string][]byte{
		"bin64/game.exe":      []byte("exe"),
		"data/a.pak":          []byte("pak"),
		"user.cfg":            []byte("cfg"),
		"screenshots/one.png": []byte("png"),
	})
	writeBranchState(t, dir, gamefiles.BranchState{Branch: "SuperTest", Files: []gamefiles.BranchFile{{Path: "bin64/game.exe"}, {Path: "data/a.pak"}}})

	if err := gameSession(t, "", false).removeClientDir(dir); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"bin64", "data", "user.cfg", "-gup-"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); err == nil {
			t.Fatalf("%s is left", gone)
		}
	}
	if readFile(t, filepath.Join(dir, "screenshots", "one.png")) != "png" {
		t.Fatal("a file the launcher does not know was removed")
	}

	if err := gameSession(t, "", true).removeClientDir(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("the folder is left after the user agreed to delete it")
	}
}
func TestRemoveMainClientKeepsOtherFoldersAndConfiguredPaths(t *testing.T) {
	parent := t.TempDir()
	vk := filepath.Join(parent, "VK")
	fx := filepath.Join(parent, "FX")
	branch := gamefiles.BranchDir(fx, "SuperTest")
	writeTree(t, vk, map[string][]byte{"-gup-/last.xml": []byte(`<Manifest Build="442"/>`)})
	writeBranchState(t, fx, gamefiles.BranchState{Branch: gamefiles.DefaultBranch, Version: "main"})
	writeBranchState(t, branch, gamefiles.BranchState{Branch: "SuperTest", Version: "test"})
	s := gameSession(t, vk, true)
	if err := s.cfg.Update(func(c *config.Config) { c.FXGame = fx }); err != nil {
		t.Fatal(err)
	}
	fxClient, err := s.FindClient(gamefiles.KindFX, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveMainClient(fxClient); err != nil {
		t.Fatal(err)
	}
	if got := installedConfiguredClients(s.cfg.Get()); len(got) != 2 || got[0].Kind != gamefiles.KindVK || got[1].Kind != gamefiles.KindBranch {
		t.Fatalf("clients after removing FX main = %+v", got)
	}
	if s.cfg.Get().FXGame != fx || s.cfg.Get().Game != vk {
		t.Fatal("configured install paths were lost")
	}
	if _, err := os.Stat(filepath.Join(branch, "-gup-", "awlauncher", "branch.json")); err != nil {
		t.Fatal("closed branch was removed:", err)
	}
}
func TestClearDownloadsRemovesPatchCache(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string][]byte{"-gup-/awlauncher-cache/payload-1-2/a.7z": []byte("patch"), "-gup-/last.xml": []byte("x")})
	if err := gameSession(t, root, true).ClearDownloads(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "-gup-", "awlauncher-cache")); err == nil {
		t.Fatal("the patch cache is left")
	}
	if readFile(t, filepath.Join(root, "-gup-", "last.xml")) != "x" {
		t.Fatal("the install state was removed")
	}
}

func writeTree(t *testing.T, root string, files map[string][]byte) {
	t.Helper()
	for name, data := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
