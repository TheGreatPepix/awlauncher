package update

import (
	"errors"
	"fmt"
	"io/fs"
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
	SelfAssetName    = "AWLauncher.exe"
	consoleAssetName = "AWLauncherConsole.exe"
	updatedFlag      = "--updated"

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
	console := filepath.Join(filepath.Dir(exe), consoleAssetName)
	if a, ok := r.asset(consoleAssetName); ok && !strings.EqualFold(console, exe) {
		if _, err := os.Stat(console); err == nil {
			files = append(files, updateFile{a, console})
		}
	}

	fmt.Printf("Updating AWLauncher %s to %s...\n", current, r.Tag)
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
			return "", fmt.Errorf("cannot replace %s: %w", f.path, err)
		}
	}
	noteInstalledVersion(exe, r.Tag)
	fmt.Printf("AWLauncher %s is installed; restarting.\n", r.Tag)
	return exe, nil
}

func swapFile(f updateFile) error {
	if err := os.Remove(f.previous()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(f.path, f.previous()); err != nil {
		return err
	}
	if err := os.Rename(f.next(), f.path); err != nil {
		_ = os.Rename(f.previous(), f.path)
		return err
	}
	return nil
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
		if h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid)); err == nil {
			windows.WaitForSingleObject(h, 20_000)
			windows.CloseHandle(h)
		}
	}
	go removePreviousFiles()
	return true
}

func removePreviousFiles() {
	exe, err := SelfPath()
	if err != nil {
		return
	}
	files := []updateFile{{path: exe}, {path: filepath.Join(filepath.Dir(exe), consoleAssetName)}}
	for attempt := 0; attempt < 10; attempt++ {
		left := false
		for _, f := range files {
			if err := os.Remove(f.previous()); err != nil && !errors.Is(err, fs.ErrNotExist) {
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
