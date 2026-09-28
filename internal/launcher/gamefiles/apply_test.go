package gamefiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallPatch(t *testing.T) {
	root := t.TempDir()
	game := filepath.Join(root, "game")
	stage := filepath.Join(root, "stage")
	backup := filepath.Join(root, "backup")
	for _, dir := range []string{filepath.Join(game, "-gup-"), stage} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	name := "bin64\\example.dat"
	target, _ := SafePath(game, name)
	staged, _ := SafePath(stage, name)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(staged), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	last := []byte(`<Manifest Build="441"><Misc/><RunCheck/></Manifest>`)
	if err := os.WriteFile(filepath.Join(game, "-gup-", "last.xml"), last, 0644); err != nil {
		t.Fatal(err)
	}
	m := Manifest{Build: 442}
	if err := installPatch(game, stage, backup, []string{name}, last, patchInfo{ModifiedUnix: 123}, m); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != "new" {
		t.Fatalf("installed %q", current)
	}
	prior, _ := SafePath(backup, name)
	old, err := os.ReadFile(prior)
	if err != nil {
		t.Fatal(err)
	}
	if string(old) != "old" {
		t.Fatalf("backup %q", old)
	}
	build, _, err := CurrentBuild(game)
	if err != nil {
		t.Fatal(err)
	}
	if build != 442 {
		t.Fatalf("build %d", build)
	}
	lastBackup := filepath.Join(backup, "-gup-", "last.xml")
	data, err := os.ReadFile(lastBackup)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `Build="441"`) {
		t.Fatalf("last.xml backup: %s", data)
	}
}

func TestInstallPatchRollsBack(t *testing.T) {
	root := t.TempDir()
	game := filepath.Join(root, "game")
	stage := filepath.Join(root, "stage")
	backup := filepath.Join(root, "backup")
	if err := os.MkdirAll(stage, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.dat", "b.dat"} {
		path, _ := SafePath(game, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("old "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	first, _ := SafePath(stage, "a.dat")
	if err := os.WriteFile(first, []byte("new a.dat"), 0644); err != nil {
		t.Fatal(err)
	}
	last := []byte(`<Manifest Build="441"/>`)
	if err := installPatch(game, stage, backup, []string{"a.dat", "b.dat"}, last, patchInfo{}, Manifest{Build: 442}); err == nil {
		t.Fatal("expected missing staged file error")
	}
	for _, name := range []string{"a.dat", "b.dat"} {
		path, _ := SafePath(game, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "old "+name {
			t.Errorf("%s = %q", name, data)
		}
	}
}
