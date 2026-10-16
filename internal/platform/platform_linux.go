package platform

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func DataDir() (string, error) {
	if root := os.Getenv("XDG_DATA_HOME"); root != "" {
		return filepath.Join(root, "awlauncher"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "awlauncher"), nil
}

func HomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

func DefaultGameDir() string { return filepath.Join(HomeDir(), "Games", "ArmoredWarfare") }

func ProtectedDirs() []string {
	dirs := []string{HomeDir(), "/home", "/usr", "/etc", "/var", "/opt", "/boot", "/run/media", "/media", "/mnt"}
	if data, err := DataDir(); err == nil {
		dirs = append(dirs, data)
	}
	return dirs
}

func Language() string {
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

func PreferredLanguages() []string {
	langs := strings.FieldsFunc(os.Getenv("LANGUAGE"), func(r rune) bool { return r == ':' })
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(env); v != "" {
			langs = append(langs, v)
		}
	}
	return langs
}

func FixedDrives() []string {
	roots := []string{HomeDir()}
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

func DiskFree(dir string) (int64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(existingDir(dir), &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}

// MemoryAvailable returns the physical memory that is free for new allocations.
func MemoryAvailable() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "MemAvailable:" {
			kib, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0
			}
			return kib << 10
		}
	}
	return 0
}

func FitCase(root, rel string) string {
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

func CloseGame(grace time.Duration) error {
	signal := func(sig syscall.Signal) (bool, error) {
		ids, err := processIDs(GameExe)
		if err != nil || len(ids) == 0 {
			return false, err
		}
		for _, id := range ids {
			if err := syscall.Kill(id, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
				return true, fmt.Errorf("cannot close %s: %w", GameExe, err)
			}
		}
		return true, nil
	}
	if running, err := signal(syscall.SIGTERM); !running || err != nil {
		return err
	}
	for deadline := time.Now().Add(grace); time.Now().Before(deadline); time.Sleep(250 * time.Millisecond) {
		if !GameRunning() {
			return nil
		}
	}
	if _, err := signal(syscall.SIGKILL); err != nil {
		return err
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(250 * time.Millisecond) {
		if !GameRunning() {
			return nil
		}
	}
	return fmt.Errorf("%s does not close", GameExe)
}

func OpenBrowser(link string) error {
	cmd := exec.Command("xdg-open", link)
	if err := cmd.Start(); err != nil {
		log.Println("Open this link in your browser:", link)
		return nil
	}
	go cmd.Wait()
	return nil
}

func runnerLog() (io.WriteCloser, string, error) {
	dir, err := DataDir()
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

func runnerCommand(exe string, args []string, dir string, extra []string) (*exec.Cmd, runner, string, error) {
	r, err := gameRunner(HomeDir(), exec.LookPath)
	if err != nil {
		return nil, r, "", err
	}
	cmd := exec.Command(r.argv[0], append(append(r.argv[1:len(r.argv):len(r.argv)], exe), args...)...)
	cmd.Dir, cmd.Env = dir, gameEnv(r.env, extra)
	out, path, err := runnerLog()
	if err != nil {
		return nil, r, "", err
	}
	cmd.Stdout, cmd.Stderr = out, out
	return cmd, r, path, nil
}

// redistMark is left in a Wine prefix once Visual C++ is installed there.
const redistMark = ".awlauncher-vcredist"

// StartGame runs the game through Proton. A new prefix gets the Visual C++
// Redistributable from the game folder first, since the game needs it.
func StartGame(exe string, args []string, dir string, env []string) (int, error) {
	cmd, r, logPath, err := runnerCommand(exe, args, dir, env)
	if err != nil {
		return 0, err
	}
	if redist := filepath.Join(dir, "vc_redist.x64.exe"); isFile(redist) && !isFile(filepath.Join(r.prefix, redistMark)) {
		cmd.Stdout.(io.Closer).Close()
		log.Print("Installing Visual C++ Redistributable into the Wine prefix first; this takes a while")
		if err := RunRedist(redist); err != nil {
			log.Printf("Visual C++ Redistributable was not installed: %v\n", err)
		}
		if cmd, r, logPath, err = runnerCommand(exe, args, dir, env); err != nil {
			return 0, err
		}
	}
	err = cmd.Start()
	cmd.Stdout.(io.Closer).Close()
	if err != nil {
		return 0, err
	}
	log.Printf("Started through %s; its output goes to %s\n", r.name, logPath)
	go cmd.Wait()
	return cmd.Process.Pid, nil
}

func RunRedist(path string) error {
	cmd, r, logPath, err := runnerCommand(path, []string{"/install", "/passive", "/norestart"}, filepath.Dir(path), nil)
	if err != nil {
		return err
	}
	defer cmd.Stdout.(io.Closer).Close()
	log.Printf("Installing through %s; the first run of a prefix may take a while. Output: %s\n", r.name, logPath)
	err = cmd.Run()
	var exit *exec.ExitError
	if err == nil || errors.As(err, &exit) && (exit.ExitCode() == 1638 || exit.ExitCode() == 3010) {
		os.WriteFile(filepath.Join(r.prefix, redistMark), nil, 0644)
	}
	return err
}

func SolidState(dir string) bool {
	var st unix.Stat_t
	if unix.Stat(existingDir(dir), &st) != nil {
		return false
	}
	block := fmt.Sprintf("/sys/dev/block/%d:%d", unix.Major(uint64(st.Dev)), unix.Minor(uint64(st.Dev)))
	for _, path := range []string{block + "/queue/rotational", block + "/../queue/rotational"} {
		if data, err := os.ReadFile(path); err == nil {
			return strings.TrimSpace(string(data)) == "0"
		}
	}
	return false
}
