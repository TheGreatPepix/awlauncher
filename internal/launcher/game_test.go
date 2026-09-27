package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeBranchState(t *testing.T, dir string, s branchState) {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, dir, map[string][]byte{"-gup-/awlauncher/branch.json": data})
}

func gameSession(t *testing.T, root string, answer bool) *session {
	t.Helper()
	t.Setenv("LOCALAPPDATA", t.TempDir())
	return &session{
		p:     prompter{confirm: func(string, bool) bool { return answer }, ask: func(string) string { return "" }},
		cfg:   newConfigStore(launcherConfig{Game: root}),
		found: &foundGame{},
	}
}

func TestInstalledClientsFindsBranchesNextToTheGame(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "Armored Warfare")
	writeTree(t, root, map[string][]byte{"-gup-/last.xml": []byte(`<Manifest Build="442"/>`)})
	writeBranchState(t, root, branchState{Branch: fxDefaultBranch, Version: "0.566.1"})
	writeBranchState(t, branchInstallDir(root, "SuperTest"), branchState{Branch: "SuperTest", Version: "0.565.1"})
	writeBranchState(t, filepath.Join(parent, "Armored Warfare copy"), branchState{Branch: "SuperTest"})
	writeTree(t, filepath.Join(parent, "Armored Warfare Mods"), map[string][]byte{"a.txt": []byte("x")})

	got := installedClients(root)
	if len(got) != 3 || got[0].Kind != clientVK || got[0].Version != "build 442" || got[1].Kind != clientFX || got[1].Version != "0.566.1" ||
		got[2].Kind != clientBranch || got[2].Branch != "SuperTest" || got[2].Dir != branchInstallDir(root, "SuperTest") {
		t.Fatalf("clients = %+v", got)
	}
}

func TestConfiguredClientFolders(t *testing.T) {
	parent := t.TempDir()
	vk := filepath.Join(parent, "VK")
	fx := filepath.Join(parent, "FX")
	branch := filepath.Join(parent, "Other drive", "SuperTest")
	writeTree(t, vk, map[string][]byte{"-gup-/last.xml": []byte(`<Manifest Build="442"/>`)})
	writeBranchState(t, vk, branchState{Branch: fxDefaultBranch, Version: "shared"})
	writeBranchState(t, fx, branchState{Branch: fxDefaultBranch, Version: "separate"})
	writeBranchState(t, branch, branchState{Branch: "SuperTest", Version: "branch"})
	cfg := launcherConfig{Game: vk, FXGame: fx, BranchGames: map[string]string{"supertest": branch}}
	if got := cfg.branchDir("SuperTest"); got != branch {
		t.Fatalf("custom branch folder = %q", got)
	}
	shared := installedConfiguredClients(cfg)
	if len(shared) != 3 || shared[0].Kind != clientVK || shared[1].Dir != vk || shared[2].Dir != branch {
		t.Fatalf("shared clients = %+v", shared)
	}
	cfg.SeparateMain = true
	separate := installedConfiguredClients(cfg)
	if len(separate) != 3 || separate[0].Kind != clientVK || separate[1].Dir != fx || separate[2].Dir != branch {
		t.Fatalf("separate clients = %+v", separate)
	}
	cfg.BranchGames = nil
	if got := cfg.branchDir("SuperTest"); got != branchInstallDir(fx, "SuperTest") {
		t.Fatalf("default branch folder in separate mode = %q", got)
	}
}

func TestChangingMainModeKeepsInstalledBranchFolder(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Armored Warfare")
	branch := branchInstallDir(root, "SuperTest")
	writeBranchState(t, branch, branchState{Branch: "SuperTest"})
	cfg := launcherConfig{Game: root}
	cfg.setSeparateMain(true)
	if got := cfg.branchDir("SuperTest"); got != branch {
		t.Fatalf("branch moved from %q to %q", branch, got)
	}
	clients := installedConfiguredClients(cfg)
	if len(clients) != 1 || clients[0].Dir != branch {
		t.Fatalf("installed branch after mode change = %+v", clients)
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
	writeBranchState(t, dir, branchState{Branch: "SuperTest", Files: []fxFile{{Path: "bin64/game.exe"}, {Path: "data/a.pak"}}})

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

func TestUninstallForgetsTheGameFolder(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Armored Warfare")
	writeTree(t, root, map[string][]byte{"data/a.pak": []byte("pak")})
	writeBranchState(t, root, branchState{Branch: fxDefaultBranch, Version: "0.566.1", Files: []fxFile{{Path: "data/a.pak"}}})
	branch := branchInstallDir(root, "SuperTest")
	writeTree(t, branch, map[string][]byte{"data/b.pak": []byte("pak")})
	writeBranchState(t, branch, branchState{Branch: "SuperTest", Files: []fxFile{{Path: "data/b.pak"}}})

	s := gameSession(t, root, true)
	if err := s.uninstallGame(); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{root, branch} {
		if _, err := os.Stat(dir); err == nil {
			t.Fatalf("%s is left", dir)
		}
	}
	if s.cfg.get().Game != "" {
		t.Fatal("the game folder is still set")
	}
}

func TestRemoveMainClientKeepsOtherFoldersAndConfiguredPaths(t *testing.T) {
	parent := t.TempDir()
	vk := filepath.Join(parent, "VK")
	fx := filepath.Join(parent, "FX")
	branch := branchInstallDir(fx, "SuperTest")
	writeTree(t, vk, map[string][]byte{"-gup-/last.xml": []byte(`<Manifest Build="442"/>`)})
	writeBranchState(t, fx, branchState{Branch: fxDefaultBranch, Version: "main"})
	writeBranchState(t, branch, branchState{Branch: "SuperTest", Version: "test"})
	s := gameSession(t, vk, true)
	if err := s.cfg.update(func(c *launcherConfig) { c.SeparateMain = true; c.FXGame = fx }); err != nil {
		t.Fatal(err)
	}
	fxClient, err := s.findClient(clientFX, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.removeMainClient(fxClient); err != nil {
		t.Fatal(err)
	}
	if got := installedConfiguredClients(s.cfg.get()); len(got) != 2 || got[0].Kind != clientVK || got[1].Kind != clientBranch {
		t.Fatalf("clients after removing FX main = %+v", got)
	}
	if s.cfg.get().FXGame != fx || s.cfg.get().Game != vk {
		t.Fatal("configured install paths were lost")
	}
	if _, err := os.Stat(branchStatePath(branch)); err != nil {
		t.Fatal("closed branch was removed:", err)
	}
}

func TestRemoveSharedMainClientRemovesBothMainClientsButKeepsBranch(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Armored Warfare")
	writeTree(t, root, map[string][]byte{"-gup-/last.xml": []byte(`<Manifest Build="442"/>`)})
	writeBranchState(t, root, branchState{Branch: fxDefaultBranch, Version: "main"})
	branch := branchInstallDir(root, "SuperTest")
	writeBranchState(t, branch, branchState{Branch: "SuperTest", Version: "test"})
	s := gameSession(t, root, true)
	var question string
	s.p.confirm = func(q string, _ bool) bool { question = q; return true }
	fxClient, err := s.findClient(clientFX, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.removeMainClient(fxClient); err != nil {
		t.Fatal(err)
	}
	if question != "Remove VK Play and FX ID main branch from "+root+"?" {
		t.Fatalf("confirmation = %q", question)
	}
	if got := installedConfiguredClients(s.cfg.get()); len(got) != 1 || got[0].Kind != clientBranch {
		t.Fatalf("clients after removing shared main folder = %+v", got)
	}
}

func TestClearDownloadsRemovesPatchCache(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string][]byte{"-gup-/awlauncher-cache/payload-1-2/a.7z": []byte("patch"), "-gup-/last.xml": []byte("x")})
	if err := gameSession(t, root, true).clearDownloads(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "-gup-", "awlauncher-cache")); err == nil {
		t.Fatal("the patch cache is left")
	}
	if readFile(t, filepath.Join(root, "-gup-", "last.xml")) != "x" {
		t.Fatal("the install state was removed")
	}
}

func TestRemovableFolderProtectsSystemFolders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	for _, dir := range []string{`C:\`, home, filepath.Join(home, "Documents"), filepath.Dir(home)} {
		if removableFolder(dir) {
			t.Errorf("%s may be deleted", dir)
		}
	}
	if !removableFolder(filepath.Join(home, "Games", "Armored Warfare")) {
		t.Error("a game folder may not be deleted")
	}
}
