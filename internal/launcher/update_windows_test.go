package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSwapFile(t *testing.T) {
	dir := t.TempDir()
	f := updateFile{path: filepath.Join(dir, "AWLauncher.exe")}
	write := func(path, text string) {
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) string {
		data, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		return string(data)
	}
	write(f.path, "old")
	write(f.next(), "new")
	write(f.previous(), "older")
	if err := swapFile(f); err != nil {
		t.Fatal(err)
	}
	if read(f.path) != "new" || read(f.previous()) != "old" || read(f.next()) != "" {
		t.Fatalf("after swap: %q %q %q", read(f.path), read(f.previous()), read(f.next()))
	}
	if filepath.Base(f.next()) != "AWLauncher-next.exe" || filepath.Base(f.previous()) != "AWLauncher-previous.exe" {
		t.Fatalf("names: %s %s", f.next(), f.previous())
	}

	os.Remove(f.next())
	if err := swapFile(f); err == nil {
		t.Fatal("swap without a download succeeded")
	}
	if read(f.path) != "new" {
		t.Fatalf("a failed swap lost the launcher: %q", read(f.path))
	}
}

func TestAfterUpdateIgnoresOtherArgs(t *testing.T) {
	if afterUpdate([]string{"AWLauncher.exe"}) || afterUpdate([]string{"AWLauncher.exe", "--other", "1"}) {
		t.Fatal("afterUpdate reacted to a normal start")
	}
}
