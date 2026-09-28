package gamefiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafePathFitsCase(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Bin64"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Bin64", "CrySystem.dll"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	path, err := SafePath(root, `BIN64\crysystem.dll`)
	if err != nil || path != filepath.Join(root, "Bin64", "CrySystem.dll") {
		t.Errorf("SafePath = %q, %v", path, err)
	}
}
