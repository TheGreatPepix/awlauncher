package launcher

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
