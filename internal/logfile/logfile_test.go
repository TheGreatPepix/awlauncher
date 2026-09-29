package logfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenStartsNewFileAndKeepsOld(t *testing.T) {
	dir := t.TempDir()
	for i := range KeepOld + 3 {
		l, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		l.Write([]byte("run " + string(rune('a'+i)) + "\n"))
		l.Close()
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != KeepOld+1 {
		t.Fatalf("files = %d, want %d", len(entries), KeepOld+1)
	}
	current, _ := os.ReadFile(filepath.Join(dir, Name))
	if !strings.HasSuffix(string(current), "run l\r\n") {
		t.Fatalf("current = %q", current)
	}
	prev, _ := os.ReadFile(filepath.Join(dir, "awlauncher.1.log"))
	if !strings.HasSuffix(string(prev), "run k\r\n") {
		t.Fatalf("previous = %q", prev)
	}
}

func TestCrashGoesToTheSessionThatCrashed(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	l.Write([]byte("before the crash"))
	l.Close()
	os.WriteFile(filepath.Join(dir, "crash.log"), []byte("panic: boom\n\ngoroutine 1\n"), 0644)
	l, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	prev, _ := os.ReadFile(filepath.Join(dir, "awlauncher.1.log"))
	if !strings.Contains(string(prev), "before the crash\r\n---- crash ----\r\npanic: boom\r\n") {
		t.Fatalf("previous = %q", prev)
	}
	if _, err := os.Stat(filepath.Join(dir, "crash.log")); !os.IsNotExist(err) {
		t.Fatal("crash.log is left behind")
	}
}

func TestWriteStampsEachLineAndRotatesBySize(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	l.Write([]byte("\none\ntwo\n"))
	text, _ := os.ReadFile(l.Path())
	lines := strings.Split(strings.TrimSuffix(string(text), "\r\n"), "\r\n")
	if len(lines) != 2 || !strings.HasSuffix(lines[0], " one") || len(lines[0]) != len(timeLayout)+4 {
		t.Fatalf("lines = %q", lines)
	}
	big := strings.Repeat("x", 1<<20)
	for range 6 {
		l.Write([]byte(big))
	}
	if _, err := os.Stat(filepath.Join(dir, "awlauncher.1.log")); err != nil {
		t.Fatal("no rotation by size:", err)
	}
	if info, _ := os.Stat(l.Path()); info.Size() > MaxSize {
		t.Fatalf("current size = %d", info.Size())
	}
}
