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
