package launcher

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
	"golang.org/x/sys/unix"
)

func focusConsole() {}

var instanceLock *os.File

func acquireInstance() (bool, error) {
	dir, err := platform.DataDir()
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "instance.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return false, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return false, nil
		}
		return false, err
	}
	instanceLock = f
	return true, nil
}

func instanceBusyMessage() string {
	return "AWLauncher is already running in another terminal or Steam session. Find it with 'pgrep -af awlauncher' and close that process before starting another."
}

func activateRunningGUI() bool { return false }

func openInShell(target string) {
	cmd := exec.Command("xdg-open", target)
	if cmd.Start() == nil {
		go cmd.Wait()
	}
}

type trayController struct{}

func newTrayController() (*trayController, error) { return nil, errors.New("no notification area") }
func (*trayController) close()                    {}
func (*trayController) hideUntilRestored() bool   { return false }
