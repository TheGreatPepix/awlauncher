package gamefiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckClientFilesDetectsDeletedAndTruncatedFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "bin64"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin64", "game.exe"), []byte("game"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "user.cfg"), []byte("FX ID config"), 0644); err != nil {
		t.Fatal(err)
	}
	inv := clientInventory{Build: 42, Files: []inventoryFile{
		{Name: `bin64\game.exe`, Size: 4},
		{Name: `bin64\missing.dll`, Size: 5},
		{Name: "user.cfg", Size: 3},
	}}
	bad, example, err := checkClientFiles(root, inv)
	if err != nil || bad != 1 || example != `bin64\missing.dll` {
		t.Fatalf("missing file: bad=%d example=%q err=%v", bad, example, err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin64", "game.exe"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	bad, _, err = checkClientFiles(root, inv)
	if err != nil || bad != 2 {
		t.Fatalf("truncated file: bad=%d err=%v", bad, err)
	}
}

func TestAllowModsPreservesChangedFileButFindsMissingFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hangar.pak"), []byte("modded hangar"), 0644); err != nil {
		t.Fatal(err)
	}
	inv := clientInventory{Files: []inventoryFile{{Name: "hangar.pak", Size: 4}, {Name: "missing.pak", Size: 4}}}
	bad, err := damagedClientFiles(root, inv, true)
	if err != nil || len(bad) != 1 || bad[0].Name != "missing.pak" {
		t.Fatalf("with mods: bad=%v, err=%v", bad, err)
	}
	bad, err = damagedClientFiles(root, inv, false)
	if err != nil || len(bad) != 2 {
		t.Fatalf("strict check: bad=%v, err=%v", bad, err)
	}
}
