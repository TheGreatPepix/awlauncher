package update

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// What AWLauncher 0.5 updated itself with: it replaced its exe through
// AWLauncher-next.exe and AWLauncher-previous.exe and started the new one
// with --updated <pid>. The bridge and the cleanup of the new launcher still
// meet those files.
const (
	legacyConsoleName = "AWLauncherConsole.exe"
	updatedFlag       = "--updated"

	installAppID = "{B06FF05E-7E61-4055-B1EA-2336934729B7}_is1"
)

type updateFile struct {
	path string
}

func (f updateFile) next() string {
	return strings.TrimSuffix(f.path, filepath.Ext(f.path)) + "-next.exe"
}
func (f updateFile) previous() string {
	return strings.TrimSuffix(f.path, filepath.Ext(f.path)) + "-previous.exe"
}

func SelfPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return exe, nil
}

var busyWait = 15 * time.Second

func whenFree(op func() error) error {
	deadline := time.Now().Add(busyWait)
	for {
		err := op()
		if err == nil || !fileBusy(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func fileBusy(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_ACCESS_DENIED)
}

// WaitForOld waits until the launcher that started this one with
// --updated <pid> has quit. It reports whether args came from an update.
func WaitForOld(args []string) bool {
	if len(args) < 3 || args[1] != updatedFlag {
		return false
	}
	if pid, err := strconv.ParseUint(args[2], 10, 32); err == nil {
		waitForPrevious(uint32(pid))
	}
	return true
}

func waitForPrevious(pid uint32) {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		h, err = windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
		if err != nil {
			return
		}
	}
	defer windows.CloseHandle(h)
	if event, _ := windows.WaitForSingleObject(h, 20_000); event != uint32(windows.WAIT_TIMEOUT) || !runsLauncher(h) {
		return
	}
	if windows.TerminateProcess(h, 1) == nil {
		windows.WaitForSingleObject(h, 5_000)
	}
}

func runsLauncher(h windows.Handle) bool {
	exe, err := SelfPath()
	if err != nil {
		return false
	}
	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &size) != nil {
		return false
	}
	image, err := os.Stat(windows.UTF16ToString(buf[:size]))
	if err != nil {
		return false
	}
	for _, path := range []string{exe, updateFile{path: exe}.previous()} {
		if info, err := os.Stat(path); err == nil && os.SameFile(image, info) {
			return true
		}
	}
	return false
}

// InstallLocation is the folder the installer put AWLauncher in, or "" when
// it was never installed.
func InstallLocation() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall\`+installAppID, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	dir, _, err := k.GetStringValue("InstallLocation")
	if err != nil {
		return ""
	}
	return filepath.Clean(dir)
}
