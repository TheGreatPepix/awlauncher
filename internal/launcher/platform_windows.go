package launcher

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func launcherDir() (string, error) {
	root := os.Getenv("LOCALAPPDATA")
	if root == "" {
		return "", errors.New("LOCALAPPDATA is not set")
	}
	return filepath.Join(root, "AWLauncher"), nil
}

func homeDir() string { return os.Getenv("USERPROFILE") }

func protectedDirs() []string {
	var dirs []string
	for _, env := range []string{"USERPROFILE", "SystemRoot", "ProgramFiles", "ProgramFiles(x86)", "ProgramData", "LOCALAPPDATA", "APPDATA"} {
		dirs = append(dirs, os.Getenv(env))
	}
	return dirs
}

func fitCase(root, rel string) string { return filepath.Join(root, rel) }

func openBrowser(link string) error {
	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", link)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot open the browser: %w", err)
	}
	return cmd.Process.Release()
}

func focusConsole() {
	if hwnd, _, _ := trayGetConsoleWindow.Call(); hwnd != 0 {
		forceForeground(hwnd)
	}
}

func startGameProcess(exe string, args []string, dir string) (int, error) {
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	return pid, cmd.Process.Release()
}

func runRedist(path string) error {
	return exec.Command("cmd", "/c", "start", "", "/wait", path, "/install", "/passive", "/norestart").Run()
}
