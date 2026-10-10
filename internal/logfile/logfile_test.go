package logfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAppendsToCurrentFile(t *testing.T) {
	dir := t.TempDir()
	for i := range 3 {
		l, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := l.Write([]byte("run " + string(rune('a'+i)) + "\n")); err != nil {
			t.Fatal(err)
		}
		if err := l.Close(); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("files = %d, want 1", len(entries))
	}
	current, err := os.ReadFile(filepath.Join(dir, Name))
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range []string{"run a\r\n", "run b\r\n", "run c\r\n"} {
		if !strings.Contains(string(current), run) {
			t.Fatalf("current is missing %q: %q", run, current)
		}
	}
}

func TestCrashIsAppendedOnNextOpen(t *testing.T) {
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
	current, _ := os.ReadFile(filepath.Join(dir, Name))
	if !strings.Contains(string(current), "before the crash\r\n---- crash ----\r\npanic: boom\r\n") {
		t.Fatalf("current = %q", current)
	}
	if _, err := os.Stat(filepath.Join(dir, "awlauncher.1.log")); !os.IsNotExist(err) {
		t.Fatal("reopening after a crash rotated the log")
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

func TestReopenFullFileRotatesOnNextWrite(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, Name)
	if err := os.WriteFile(current, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(current, MaxSize); err != nil {
		t.Fatal(err)
	}
	l, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if _, err := os.Stat(filepath.Join(dir, "awlauncher.1.log")); !os.IsNotExist(err) {
		t.Fatal("opening a full log rotated it before a write")
	}
	l.Write([]byte("next run"))
	previous, err := os.Stat(filepath.Join(dir, "awlauncher.1.log"))
	if err != nil || previous.Size() != MaxSize {
		t.Fatalf("previous log = %v, %v", previous, err)
	}
	content, err := os.ReadFile(current)
	if err != nil || !strings.HasSuffix(string(content), "next run\r\n") {
		t.Fatalf("current log = %q, %v", content, err)
	}
}
