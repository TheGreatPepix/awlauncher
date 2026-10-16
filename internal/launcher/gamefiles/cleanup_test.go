package gamefiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveClientFilesRemovesACancelledInstall(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string][]byte{
		"bin64/game.exe":    []byte("exe"),
		"data/a.pak.part":   []byte("pa"),
		"notes/mine.txt":    []byte("mine"),
		"-gup-/other/x.bin": []byte("x"),
	})
	if err := markPending(root, []remoteFile{{Path: "bin64/game.exe"}, {Path: "data/a.pak"}, {Path: "data/b.pak"}}); err != nil {
		t.Fatal(err)
	}
	if err := RemoveClientFiles(root); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"bin64", "data", "-gup-"} {
		if _, err := os.Stat(filepath.Join(root, gone)); err == nil {
			t.Errorf("%s is left", gone)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "notes", "mine.txt")); err != nil {
		t.Fatal("a file the install did not write was removed:", err)
	}
}

func TestClearPendingAfterInstall(t *testing.T) {
	root := t.TempDir()
	if err := markPending(root, []remoteFile{{Path: "a"}}); err != nil {
		t.Fatal(err)
	}
	clearPending(root)
	if got := readPending(root); len(got) != 0 {
		t.Fatalf("pending = %v", got)
	}
}
