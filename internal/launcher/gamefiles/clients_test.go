package gamefiles

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestInstalledClientsFindsBranchesNextToTheGame(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "Armored Warfare")
	writeTree(t, root, map[string][]byte{"-gup-/last.xml": []byte(`<Manifest Build="442"/>`)})
	writeBranchState(t, root, BranchState{Branch: DefaultBranch, Version: "0.566.1"})
	writeBranchState(t, BranchDir(root, "SuperTest"), BranchState{Branch: "SuperTest", Version: "0.565.1"})
	writeBranchState(t, filepath.Join(parent, "Armored Warfare copy"), BranchState{Branch: "SuperTest"})
	writeTree(t, filepath.Join(parent, "Armored Warfare Mods"), map[string][]byte{"a.txt": []byte("x")})

	got := InstalledClients(root)
	if len(got) != 3 || got[0].Kind != KindVK || got[0].Version != "build 442" || got[1].Kind != KindFX || got[1].Version != "0.566.1" ||
		got[2].Kind != KindBranch || got[2].Branch != "SuperTest" || got[2].Dir != BranchDir(root, "SuperTest") {
		t.Fatalf("clients = %+v", got)
	}
}

func TestRemovableFolderProtectsSystemFolders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	for _, dir := range []string{`C:\`, home, filepath.Join(home, "Documents"), filepath.Dir(home)} {
		if RemovableFolder(dir) {
			t.Errorf("%s may be deleted", dir)
		}
	}
	if !RemovableFolder(filepath.Join(home, "Games", "Armored Warfare")) {
		t.Error("a game folder may not be deleted")
	}
}

func writeBranchState(t *testing.T, dir string, s BranchState) {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, dir, map[string][]byte{"-gup-/awlauncher/branch.json": data})
}

func TestVKClientShowsItsGameVersion(t *testing.T) {
	for _, tc := range []struct {
		files map[string][]byte
		want  string
	}{
		{map[string][]byte{"build_info.txt": []byte("0.566.1/9d9f975\r\n"), "build_version.txt": []byte("0.1.1--2020-01-01_00:00")}, "0.566.1"},
		{map[string][]byte{"build_version.txt": []byte("0.566.1--2026-09-25_03:27")}, "0.566.1"},
		{map[string][]byte{"build_info.txt": []byte("staging/abc")}, "build 442"},
		{map[string][]byte{}, "build 442"},
	} {
		root := t.TempDir()
		tc.files["-gup-/last.xml"] = []byte(`<Manifest Build="442"/>`)
		writeTree(t, root, tc.files)
		if got := InstalledClients(root); len(got) != 1 || got[0].Version != tc.want {
			t.Fatalf("files %v: clients = %+v, want %q", tc.files, got, tc.want)
		}
	}
}
