package update

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/progress"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	SelfAssetName     = "AWLauncher.exe"
	legacyConsoleName = "AWLauncherConsole.exe"
	updatedFlag       = "--updated"

	installAppID = "{B06FF05E-7E61-4055-B1EA-2336934729B7}_is1"
)

type updateFile struct {
	asset releaseAsset
	path  string
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

func Install(client *http.Client, current string) (string, error) {
	r, err := latestRelease(client, current)
	if err != nil {
		return "", err
	}
	if !newerRelease(r.Tag, current) {
		return "", fmt.Errorf("AWLauncher %s is up to date", current)
	}
	exe, err := SelfPath()
	if err != nil {
		return "", err
	}
	main, ok := r.asset(SelfAssetName)
	if !ok {
		return "", fmt.Errorf("release %s has no %s; download it from %s", r.Tag, SelfAssetName, r.Page)
	}
	files := []updateFile{{main, exe}}

	log.Printf("Updating AWLauncher %s to %s...\n", current, r.Tag)
	var total int64
	for _, f := range files {
		total += f.asset.Size
	}
	progress.Default.Begin("Downloading AWLauncher "+r.Tag, progress.UnitBytes, total, 0)
	for _, f := range files {
		if err := downloadAsset(client, f.asset, f.next()); err != nil {
			progress.Default.End()
			if errors.Is(err, fs.ErrPermission) {
				return "", fmt.Errorf("AWLauncher cannot write to %s; download %s from %s", filepath.Dir(exe), r.Tag, r.Page)
			}
			return "", err
		}
	}
	progress.Default.End()

	for i, f := range files {
		if err := swapFile(f); err != nil {
			for _, done := range files[:i] {
				_ = os.Rename(done.path, done.next())
				_ = os.Rename(done.previous(), done.path)
			}
			for _, rest := range files[i:] {
				os.Remove(rest.next())
			}
			if fileBusy(err) {
				return "", fmt.Errorf("cannot replace %s: %w; another program, often an antivirus scan, keeps the new file open, so try again in a minute", f.path, err)
			}
			return "", fmt.Errorf("cannot replace %s: %w", f.path, err)
		}
	}
	noteInstalledVersion(exe, r.Tag)
	log.Printf("AWLauncher %s is installed; restarting.\n", r.Tag)
	return exe, nil
}

func swapFile(f updateFile) error {
	err := whenFree(func() error {
		if err := os.Remove(f.previous()); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := whenFree(func() error { return os.Rename(f.path, f.previous()) }); err != nil {
		return err
	}
	if err := whenFree(func() error { return os.Rename(f.next(), f.path) }); err != nil {
		_ = whenFree(func() error { return os.Rename(f.previous(), f.path) })
		return err
	}
	return nil
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

func StartUpdated(exe string, allowForeground func(int)) error {
	cmd := exec.Command(exe, updatedFlag, strconv.Itoa(os.Getpid()))
	cmd.Dir = filepath.Dir(exe)
	if err := cmd.Start(); err != nil {
		return err
	}
	allowForeground(cmd.Process.Pid)
	return cmd.Process.Release()
}

func AfterUpdate(args []string) bool {
	if len(args) < 3 || args[1] != updatedFlag {
		return false
	}
	if pid, err := strconv.ParseUint(args[2], 10, 32); err == nil {
		waitForPrevious(uint32(pid))
	}
	go removePreviousFiles()
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

func removePreviousFiles() {
	exe, err := SelfPath()
	if err != nil {
		return
	}
	console := updateFile{path: filepath.Join(filepath.Dir(exe), legacyConsoleName)}
	stale := []string{updateFile{path: exe}.previous(), console.previous(), console.next(), console.path}
	for attempt := 0; attempt < 10; attempt++ {
		left := false
		for _, path := range stale {
			if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				left = true
			}
		}
		if !left {
			return
		}
		time.Sleep(time.Second)
	}
}

func noteInstalledVersion(exe, tag string) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall\`+installAppID, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	dir, _, err := k.GetStringValue("InstallLocation")
	if err != nil || !strings.EqualFold(filepath.Clean(dir), filepath.Dir(exe)) {
		return
	}
	_ = k.SetStringValue("DisplayVersion", strings.TrimPrefix(tag, "v"))
}
