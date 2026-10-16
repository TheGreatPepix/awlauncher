package update

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWaitForOldIgnoresOtherArgs(t *testing.T) {
	if WaitForOld([]string{"AWLauncher.exe"}) || WaitForOld([]string{"AWLauncher.exe", "--other", "1"}) {
		t.Fatal("WaitForOld reacted to a normal start")
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

func TestStepAside(t *testing.T) {
	dir := t.TempDir()
	f := updateFile{path: filepath.Join(dir, "AWLauncher.exe")}
	os.WriteFile(f.path, []byte("bridge"), 0644)
	os.WriteFile(f.previous(), []byte("0.5"), 0644)
	h := holdOpen(t, f.previous())
	time.AfterFunc(600*time.Millisecond, func() { windows.CloseHandle(h) })
	if err := StepAside(f.path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(f.path); err == nil {
		t.Fatal("the bridge is still in the installer's way")
	}
	if data, _ := os.ReadFile(f.previous()); string(data) != "bridge" {
		t.Fatalf("the previous file is %q", data)
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
