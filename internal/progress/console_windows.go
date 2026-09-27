package progress

import (
	"fmt"
	"syscall"
	"unsafe"
)

func ClearScreen() {
	if !ui.live || !consoleInput() {
		return
	}
	fmt.Print("\x1b[2J\x1b[H")
}

func InteractiveScreen() bool { return ui.live && consoleInput() }

func consoleInput() bool {
	h, err := syscall.GetStdHandle(syscall.STD_INPUT_HANDLE)
	if err != nil {
		return false
	}
	var mode uint32
	ok, _, _ := getConsoleMode.Call(uintptr(h), uintptr(unsafe.Pointer(&mode)))
	return ok != 0
}

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode             = kernel32.NewProc("GetConsoleMode")
	setConsoleMode             = kernel32.NewProc("SetConsoleMode")
	getConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

func enableVT() bool {
	h, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil {
		return false
	}
	var mode uint32
	if ok, _, _ := getConsoleMode.Call(uintptr(h), uintptr(unsafe.Pointer(&mode))); ok == 0 {
		return false
	}
	const virtualTerminalProcessing = 0x0004
	if mode&virtualTerminalProcessing != 0 {
		return true
	}
	ok, _, _ := setConsoleMode.Call(uintptr(h), uintptr(mode|virtualTerminalProcessing))
	return ok != 0
}

func consoleWidth() int {
	h, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil {
		return 80
	}
	var info struct {
		size, cursor             [2]int16
		attributes               uint16
		left, top, right, bottom int16
		maxSize                  [2]int16
	}
	if ok, _, _ := getConsoleScreenBufferInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&info))); ok == 0 {
		return 80
	}
	if w := int(info.right-info.left) + 1; w > 20 {
		return w
	}
	return 80
}
