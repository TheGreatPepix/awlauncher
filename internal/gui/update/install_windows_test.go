package update

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
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
	if AfterUpdate([]string{"AWLauncher.exe"}) || AfterUpdate([]string{"AWLauncher.exe", "--other", "1"}) {
		t.Fatal("afterUpdate reacted to a normal start")
	}
}

func holdOpen(t *testing.T, path string) windows.Handle {
	t.Helper()
	name, _ := windows.UTF16PtrFromString(path)
	h, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestSwapFileWaitsForABusyDownload(t *testing.T) {
	dir := t.TempDir()
	f := updateFile{path: filepath.Join(dir, "AWLauncher.exe")}
	os.WriteFile(f.path, []byte("old"), 0644)
	os.WriteFile(f.next(), []byte("new"), 0644)
	h := holdOpen(t, f.next())
	time.AfterFunc(600*time.Millisecond, func() { windows.CloseHandle(h) })
	if err := swapFile(f); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(f.path); string(data) != "new" {
		t.Fatalf("launcher is %q after the swap", data)
	}
}

func TestSwapFileKeepsTheLauncherWhenTheDownloadStaysBusy(t *testing.T) {
	old := busyWait
	busyWait = 300 * time.Millisecond
	t.Cleanup(func() { busyWait = old })
	dir := t.TempDir()
	f := updateFile{path: filepath.Join(dir, "AWLauncher.exe")}
	os.WriteFile(f.path, []byte("old"), 0644)
	os.WriteFile(f.next(), []byte("new"), 0644)
	h := holdOpen(t, f.next())
	defer windows.CloseHandle(h)
	err := swapFile(f)
	if err == nil || !fileBusy(err) {
		t.Fatalf("swap of a busy file: %v", err)
	}
	if data, _ := os.ReadFile(f.path); string(data) != "old" {
		t.Fatalf("a failed swap left the launcher as %q", data)
	}
}

func TestRunsLauncherRecognizesItself(t *testing.T) {
	if !runsLauncher(windows.CurrentProcess()) {
		t.Fatal("the current process is not recognized")
	}
	parent, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(os.Getppid()))
	if err != nil {
		t.Skip("cannot open the parent process: ", err)
	}
	defer windows.CloseHandle(parent)
	if runsLauncher(parent) {
		t.Fatal("the parent process is taken for the launcher")
	}
}
