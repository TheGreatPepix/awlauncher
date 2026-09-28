package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	user32   = windows.NewLazySystemDLL("user32.dll")
	crypt32  = windows.NewLazySystemDLL("crypt32.dll")

	procCryptProtectData         = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData       = crypt32.NewProc("CryptUnprotectData")
	procLocalFree                = kernel32.NewProc("LocalFree")
	procGetConsoleProcessList    = kernel32.NewProc("GetConsoleProcessList")
	procGetDiskFreeSpaceExW      = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGetDriveTypeW            = kernel32.NewProc("GetDriveTypeW")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procPostMessageW             = user32.NewProc("PostMessageW")
)

const wmClose = 0x0010

func DataDir() (string, error) {
	root := os.Getenv("LOCALAPPDATA")
	if root == "" {
		return "", errors.New("LOCALAPPDATA is not set")
	}
	return filepath.Join(root, "AWLauncher"), nil
}

func HomeDir() string { return os.Getenv("USERPROFILE") }

func DefaultGameDir() string { return `C:\Games\Armored Warfare` }

func ProtectedDirs() []string {
	var dirs []string
	for _, env := range []string{"USERPROFILE", "SystemRoot", "ProgramFiles", "ProgramFiles(x86)", "ProgramData", "LOCALAPPDATA", "APPDATA"} {
		dirs = append(dirs, os.Getenv(env))
	}
	return dirs
}

func FitCase(root, rel string) string { return filepath.Join(root, rel) }

func OpenBrowser(link string) error {
	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", link)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot open the browser: %w", err)
	}
	return cmd.Process.Release()
}

func StartGame(exe string, args []string, dir string) (int, error) {
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	return pid, cmd.Process.Release()
}

func RunRedist(path string) error {
	return exec.Command("cmd", "/c", "start", "", "/wait", path, "/install", "/passive", "/norestart").Run()
}

type dataBlob struct {
	size uint32
	data *byte
}

func userCrypt(input []byte, proc *windows.LazyProc) ([]byte, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("empty credentials")
	}
	in := dataBlob{size: uint32(len(input)), data: &input[0]}
	var out dataBlob
	ok, _, callErr := proc.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&out)))
	if ok == 0 {
		return nil, fmt.Errorf("Windows DPAPI: %w", callErr)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.data)))
	return append([]byte(nil), unsafe.Slice(out.data, out.size)...), nil
}

func Protect(data []byte) ([]byte, error)   { return userCrypt(data, procCryptProtectData) }
func Unprotect(data []byte) ([]byte, error) { return userCrypt(data, procCryptUnprotectData) }

func OwnsConsole() bool {
	var pids [2]uint32
	n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), 2)
	return n == 1
}

func FixedDrives() []string {
	var roots []string
	for letter := 'C'; letter <= 'Z'; letter++ {
		root := string(letter) + `:\`
		name, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		if kind, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(name))); kind == 3 {
			roots = append(roots, root)
		}
	}
	return roots
}

func DiskFree(dir string) (int64, error) {
	dir = existingDir(dir)
	path, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var free uint64
	ok, _, callErr := procGetDiskFreeSpaceExW.Call(uintptr(unsafe.Pointer(path)), uintptr(unsafe.Pointer(&free)), 0, 0)
	if ok == 0 {
		return 0, callErr
	}
	return int64(free), nil
}

func processIDs(name string) ([]uint32, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot list running processes: %w", err)
	}
	defer windows.CloseHandle(snap)
	var ids []uint32
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snap, &entry); err == nil; err = windows.Process32Next(snap, &entry) {
		if strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), name) {
			ids = append(ids, entry.ProcessID)
		}
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, fmt.Errorf("cannot list running processes: %w", err)
	}
	return ids, nil
}

func CloseGame(grace time.Duration) error {
	ids, err := processIDs(GameExe)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	wanted := map[uint32]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	enum := windows.NewCallback(func(hwnd, _ uintptr) uintptr {
		var pid uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if wanted[pid] {
			procPostMessageW.Call(hwnd, wmClose, 0, 0)
		}
		return 1
	})
	procEnumWindows.Call(enum, 0)
	for deadline := time.Now().Add(grace); time.Now().Before(deadline); time.Sleep(250 * time.Millisecond) {
		if !GameRunning() {
			return nil
		}
	}
	ids, err = processIDs(GameExe)
	if err != nil {
		return err
	}
	for _, id := range ids {
		h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, id)
		if err != nil {
			return fmt.Errorf("cannot close %s: %w", GameExe, err)
		}
		err = windows.TerminateProcess(h, 1)
		if err == nil {
			_, _ = windows.WaitForSingleObject(h, 5000)
		}
		windows.CloseHandle(h)
		if err != nil {
			return fmt.Errorf("cannot close %s: %w", GameExe, err)
		}
	}
	return nil
}
