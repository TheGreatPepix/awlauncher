package launcher

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

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

func acquireInstance() (bool, error) {
	name, _ := windows.UTF16PtrFromString(instanceMutex)
	h, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	singleInstance = h
	return true, nil
}

func instanceBusyMessage() string {
	return "AWLauncher is already running. Close it first (tray icon, Exit)"
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

func focusConsole() {
	if hwnd, _, _ := trayGetConsoleWindow.Call(); hwnd != 0 {
		forceForeground(hwnd)
	}
}
