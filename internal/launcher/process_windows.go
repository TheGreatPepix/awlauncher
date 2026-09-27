package launcher

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const gameExe = "ArmoredWarfare.exe"

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

var (
	procEnumWindows              = winUser32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = winUser32.NewProc("GetWindowThreadProcessId")
)

func closeGame(grace time.Duration) error {
	ids, err := processIDs(gameExe)
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
		if !gameRunning() {
			return nil
		}
	}
	ids, err = processIDs(gameExe)
	if err != nil {
		return err
	}
	for _, id := range ids {
		h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, id)
		if err != nil {
			return fmt.Errorf("cannot close %s: %w", gameExe, err)
		}
		err = windows.TerminateProcess(h, 1)
		if err == nil {
			_, _ = windows.WaitForSingleObject(h, 5000)
		}
		windows.CloseHandle(h)
		if err != nil {
			return fmt.Errorf("cannot close %s: %w", gameExe, err)
		}
	}
	return nil
}

const (
	instanceMutex = `Local\AWLauncher.SingleInstance`
	showMessage   = "AWLauncher.Show"
)

var (
	procRegisterWindowMessageW  = winUser32.NewProc("RegisterWindowMessageW")
	procFindWindowW             = winUser32.NewProc("FindWindowW")
	procAllowSetForegroundWindo = winUser32.NewProc("AllowSetForegroundWindow")
	singleInstance              windows.Handle
)

func acquireInstance() bool {
	name, _ := windows.UTF16PtrFromString(instanceMutex)
	h, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return false
	}
	singleInstance = h
	return true
}

func showWindowMessage() uint32 {
	name, _ := windows.UTF16PtrFromString(showMessage)
	id, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(name)))
	return uint32(id)
}

func activateRunningGUI() bool {
	class, _ := windows.UTF16PtrFromString(hostClassName)
	hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(class)), 0)
	if hwnd == 0 {
		return false
	}
	procAllowSetForegroundWindo.Call(^uintptr(0))
	procPostMessageW.Call(hwnd, uintptr(showWindowMessage()), 0, 0)
	return true
}
