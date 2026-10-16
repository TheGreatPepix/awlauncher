package update

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

// On Linux the launcher is one AppImage file, which updates by replacing
// itself: the AppImage runtime names it in APPIMAGE.

// SetupAssetName is the AppImage the launcher updates itself with.
const SetupAssetName = "AWLauncher-x86_64.AppImage"

func canInstall() bool { return os.Getenv("APPIMAGE") != "" }

func appImage() (string, error) {
	path := os.Getenv("APPIMAGE")
	if path == "" {
		return "", errors.New("only the AppImage of AWLauncher can update itself")
	}
	return path, nil
}

// DownloadSetup downloads the newest AppImage next to the running one.
func DownloadSetup(client *http.Client, current string) (string, error) {
	target, err := appImage()
	if err != nil {
		return "", err
	}
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
	next := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".next")
	log.Printf("Updating AWLauncher %s to %s...\n", current, r.Tag)
	progress.Default.Begin("Downloading AWLauncher "+r.Tag, progress.UnitBytes, a.Size, 0)
	err = downloadAsset(client, a, next)
	progress.Default.End()
	if err != nil {
		os.Remove(next)
		return "", err
	}
	return next, nil
}

// RunSetup puts the downloaded AppImage in place of the running one and
// starts it once the window, the core's parent, has quit.
func RunSetup(setup, _ string, _ bool) error {
	target, err := appImage()
	if err != nil {
		return err
	}
	if err := os.Chmod(setup, 0755); err != nil {
		return err
	}
	if err := os.Rename(setup, target); err != nil {
		return err
	}
	wait := `while kill -0 "$1" 2>/dev/null; do sleep 0.2; done; exec "$0" --updated`
	cmd := exec.Command("/bin/sh", "-c", wait, target, strconv.Itoa(os.Getppid()))
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot restart AWLauncher: %w", err)
	}
	return cmd.Process.Release()
}

func RemoveLeftovers(string) {}
