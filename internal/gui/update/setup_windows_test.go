package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOwnFolder(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"AWLauncher.exe", "AWLauncher-previous.exe"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !OwnFolder(dir) {
		t.Fatal("a folder with only the launcher is not its own")
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if OwnFolder(dir) {
		t.Fatal("a folder with other files counts as the launcher's own")
	}
}

func TestRemoveLeftovers(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "AWLauncher.exe")
	keep := []string{exe, filepath.Join(dir, "awcore.exe")}
	gone := []string{
		filepath.Join(dir, "AWLauncher-previous.exe"),
		filepath.Join(dir, "AWLauncher-next.exe"),
		filepath.Join(dir, "AWLauncherConsole.exe"),
	}
	for _, path := range append(keep, gone...) {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	RemoveLeftovers(exe)
	for _, path := range keep {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s is gone", filepath.Base(path))
		}
	}
	for _, path := range gone {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s is left", filepath.Base(path))
		}
	}
}
