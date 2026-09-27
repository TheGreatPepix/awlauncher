package launcher

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const (
	gameExe       = "ArmoredWarfare.exe"
	selfAssetName = "awlauncher-linux-amd64"
)

func launcherDir() (string, error) {
	if root := os.Getenv("XDG_DATA_HOME"); root != "" {
		return filepath.Join(root, "awlauncher"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "awlauncher"), nil
}

func homeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

func defaultGameDir() string { return filepath.Join(homeDir(), "Games", "ArmoredWarfare") }

func protectedDirs() []string {
	dirs := []string{homeDir(), "/home", "/usr", "/etc", "/var", "/opt", "/boot", "/run/media", "/media", "/mnt"}
	if data, err := launcherDir(); err == nil {
		dirs = append(dirs, data)
	}
	return dirs
}

func uiLanguage() string {
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(env); v != "" {
			if strings.HasPrefix(strings.ToLower(v), "ru") {
				return "ru"
			}
			return "en"
		}
	}
	return "en"
}

func protectForUser(data []byte) ([]byte, error)   { return data, nil }
func unprotectForUser(data []byte) ([]byte, error) { return data, nil }

func ownsConsole() bool { return false }

func focusConsole() {}

func fixedDrives() []string {
	roots := []string{homeDir()}
	for _, pattern := range []string{"/run/media/*", "/run/media/*/*", "/media/*", "/media/*/*", "/mnt/*"} {
		matches, _ := filepath.Glob(pattern)
		for _, m := range matches {
			if st, err := os.Stat(m); err == nil && st.IsDir() {
				roots = append(roots, m)
			}
		}
	}
	return roots
}

func diskFree(dir string) (int64, error) {
	for {
		if _, err := os.Stat(dir); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}

func fitCase(root, rel string) string {
	full := filepath.Join(root, rel)
	if _, err := os.Lstat(full); err == nil {
		return full
	}
	parts := strings.Split(rel, string(filepath.Separator))
	dir := root
	for i, part := range parts {
		next := filepath.Join(dir, part)
		if _, err := os.Lstat(next); err != nil {
			entries, err := os.ReadDir(dir)
			found := false
			for _, e := range entries {
				if err == nil && strings.EqualFold(e.Name(), part) {
					next, found = filepath.Join(dir, e.Name()), true
					break
				}
			}
			if !found {
				return filepath.Join(append([]string{dir}, parts[i:]...)...)
			}
		}
		dir = next
	}
	return dir
}

func processIDs(name string) ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("cannot list running processes: %w", err)
	}
	var ids []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		cmdline, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil {
			continue
		}
		if strings.EqualFold(programName(cmdline), name) {
			ids = append(ids, pid)
		}
	}
	return ids, nil
}

func programName(cmdline []byte) string {
	argv0, _, _ := strings.Cut(string(cmdline), "\x00")
	if argv0 == "" {
		return ""
	}
	return filepath.Base(strings.ReplaceAll(argv0, `\`, "/"))
}

func runningProcess(names ...string) (string, error) {
	for _, name := range names {
		ids, err := processIDs(name)
		if err != nil {
			return "", err
		}
		if len(ids) > 0 {
			return name, nil
		}
	}
	return "", nil
}

func gameRunning() bool {
	ids, err := processIDs(gameExe)
	return err == nil && len(ids) > 0
}

func closeGame(grace time.Duration) error {
	signal := func(sig syscall.Signal) (bool, error) {
		ids, err := processIDs(gameExe)
		if err != nil || len(ids) == 0 {
			return false, err
		}
		for _, id := range ids {
			if err := syscall.Kill(id, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
				return true, fmt.Errorf("cannot close %s: %w", gameExe, err)
			}
		}
		return true, nil
	}
	if running, err := signal(syscall.SIGTERM); !running || err != nil {
		return err
	}
	for deadline := time.Now().Add(grace); time.Now().Before(deadline); time.Sleep(250 * time.Millisecond) {
		if !gameRunning() {
			return nil
		}
	}
	if _, err := signal(syscall.SIGKILL); err != nil {
		return err
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(250 * time.Millisecond) {
		if !gameRunning() {
			return nil
		}
	}
	return fmt.Errorf("%s does not close", gameExe)
}

var instanceLock *os.File

func acquireInstance() (bool, error) {
	dir, err := launcherDir()
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

func openBrowser(link string) error {
	cmd := exec.Command("xdg-open", link)
	if err := cmd.Start(); err != nil {
		fmt.Println("Open this link in your browser:", link)
		return nil
	}
	go cmd.Wait()
	return nil
}

type trayController struct{}

func newTrayController() (*trayController, error) { return nil, errors.New("no notification area") }
func (*trayController) close()                    {}
func (*trayController) hideUntilRestored() bool   { return false }

func gameRunner() ([]string, []string, error) {
	var runner []string
	if custom := strings.Fields(os.Getenv("AWLAUNCHER_RUNNER")); len(custom) > 0 {
		runner = custom
	} else if path, err := exec.LookPath("umu-run"); err == nil {
		runner = []string{path}
	} else if path, err := exec.LookPath("wine"); err == nil {
		runner = []string{path}
	} else {
		return nil, nil, errors.New("nothing can run the Windows game here: install umu-launcher (umu-run) or Wine, or set AWLAUNCHER_RUNNER")
	}
	env := os.Environ()
	if os.Getenv("WINEPREFIX") == "" {
		dir, err := launcherDir()
		if err != nil {
			return nil, nil, err
		}
		prefix := filepath.Join(dir, "prefix")
		if err := os.MkdirAll(prefix, 0755); err != nil {
			return nil, nil, err
		}
		env = append(env, "WINEPREFIX="+prefix)
	}
	if strings.HasPrefix(filepath.Base(runner[0]), "umu-run") && os.Getenv("GAMEID") == "" {
		env = append(env, "GAMEID=umu-default")
	}
	return runner, env, nil
}

func runnerLog() (io.WriteCloser, string, error) {
	dir, err := launcherDir()
	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, "game.log")
	f, err := os.Create(path)
	return f, path, err
}

func runnerCommand(exe string, args []string, dir string) (*exec.Cmd, string, error) {
	runner, env, err := gameRunner()
	if err != nil {
		return nil, "", err
	}
	cmd := exec.Command(runner[0], append(append(runner[1:len(runner):len(runner)], exe), args...)...)
	cmd.Dir, cmd.Env = dir, env
	log, path, err := runnerLog()
	if err != nil {
		return nil, "", err
	}
	cmd.Stdout, cmd.Stderr = log, log
	return cmd, path, nil
}

func startGameProcess(exe string, args []string, dir string) (int, error) {
	cmd, logPath, err := runnerCommand(exe, args, dir)
	if err != nil {
		return 0, err
	}
	err = cmd.Start()
	cmd.Stdout.(io.Closer).Close()
	if err != nil {
		return 0, err
	}
	fmt.Printf("Started through %s; its output goes to %s\n", filepath.Base(cmd.Path), logPath)
	go cmd.Wait()
	return cmd.Process.Pid, nil
}

func runRedist(path string) error {
	cmd, logPath, err := runnerCommand(path, []string{"/install", "/passive", "/norestart"}, filepath.Dir(path))
	if err != nil {
		return err
	}
	defer cmd.Stdout.(io.Closer).Close()
	fmt.Printf("Installing through %s; the first run may download Proton. Output: %s\n", filepath.Base(cmd.Path), logPath)
	return cmd.Run()
}
