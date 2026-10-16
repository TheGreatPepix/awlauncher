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
	"strings"
	"syscall"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

// The Flutter launcher is a folder, so it updates through its installer:
// the core downloads it, the window and the core quit, and the installer
// waits for them, installs the new version over the folder and starts it.

// setupUpdateFlag tells the installer that a launcher started it: it waits
// for that launcher to quit, asks nothing and starts the new version.
const setupUpdateFlag = "/UPDATE"

// DownloadSetup downloads the installer of the newest release into a new
// temporary folder.
func DownloadSetup(client *http.Client, current string) (string, error) {
	r, err := latestRelease(client, current)
	if err != nil {
		return "", err
	}
	if !newerRelease(r.Tag, current) {
		return "", fmt.Errorf("AWLauncher %s is up to date", current)
	}
	a, ok := r.asset(SetupAssetName)
	if !ok {
		return "", fmt.Errorf("release %s has no %s; download it from %s", r.Tag, SetupAssetName, r.Page)
	}
	dir, err := os.MkdirTemp("", "AWLauncher-update-")
	if err != nil {
		return "", err
	}
	setup := filepath.Join(dir, SetupAssetName)
	log.Printf("Updating AWLauncher %s to %s...\n", current, r.Tag)
	progress.Default.Begin("Downloading AWLauncher "+r.Tag, progress.UnitBytes, a.Size, 0)
	err = downloadAsset(client, a, setup)
	progress.Default.End()
	if err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return setup, nil
}

// RunSetup starts the installer for the launcher in dir, or in the folder
// it was installed to when dir is "". With visible the installer shows its
// progress; otherwise it shows nothing.
func RunSetup(setup, dir string, visible bool) error {
	line := fmt.Sprintf(`"%s" /VERYSILENT`, setup)
	if visible {
		line = fmt.Sprintf(`"%s" /SILENT`, setup)
	}
	line += " /SUPPRESSMSGBOXES /NORESTART"
	if dir != "" {
		// Inno Setup reads its arguments itself and takes /DIR="..." quoted
		// this way, which exec would quote differently.
		line += fmt.Sprintf(` /DIR="%s"`, filepath.Clean(dir))
	}
	cmd := exec.Command(setup)
	cmd.Dir = filepath.Dir(setup)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: line + " " + setupUpdateFlag}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot start the installer: %w", err)
	}
	return cmd.Process.Release()
}

// StepAside moves the running exe out of the installer's way. Windows lets
// a running program be renamed but not replaced.
func StepAside(exe string) error {
	f := updateFile{path: exe}
	if err := whenFree(func() error {
		if err := os.Remove(f.previous()); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return os.Rename(exe, f.previous())
}

// OwnFolder reports whether dir holds nothing but AWLauncher, so that the
// Flutter launcher's files can go there: the folder it was installed to, or
// one with only the WebView launcher's files in it.
func OwnFolder(dir string) bool {
	if installed := InstallLocation(); installed != "" && strings.EqualFold(installed, filepath.Clean(dir)) {
		return true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if e.IsDir() || !strings.HasPrefix(name, "awlauncher") || !strings.HasSuffix(name, ".exe") {
			return false
		}
	}
	return true
}

// RemoveLeftovers deletes what earlier versions left next to exe: the
// WebView launcher's previous and next files and the one that moved it to
// this version. They may still be running for a few seconds.
func RemoveLeftovers(exe string) {
	main := updateFile{path: exe}
	console := updateFile{path: filepath.Join(filepath.Dir(exe), legacyConsoleName)}
	stale := []string{main.previous(), main.next(), console.previous(), console.next(), console.path}
	for attempt := 0; attempt < 30; attempt++ {
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
