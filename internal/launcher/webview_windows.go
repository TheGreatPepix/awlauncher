package launcher

import (
	"errors"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	winUser32 = windows.NewLazySystemDLL("user32.dll")
	kernel32  = windows.NewLazySystemDLL("kernel32.dll")

	procGetWindowThreadProcessId = winUser32.NewProc("GetWindowThreadProcessId")
	winDwmapi                    = windows.NewLazySystemDLL("dwmapi.dll")
	winShell32                   = windows.NewLazySystemDLL("shell32.dll")
	winOle32                     = windows.NewLazySystemDLL("ole32.dll")

	procRegisterClassExW         = winUser32.NewProc("RegisterClassExW")
	procCreateWindowExW          = winUser32.NewProc("CreateWindowExW")
	procDefWindowProcW           = winUser32.NewProc("DefWindowProcW")
	procDestroyWindow            = winUser32.NewProc("DestroyWindow")
	procShowWindow               = winUser32.NewProc("ShowWindow")
	procIsWindowVisible          = winUser32.NewProc("IsWindowVisible")
	procIsIconic                 = winUser32.NewProc("IsIconic")
	procSetForegroundWindow      = winUser32.NewProc("SetForegroundWindow")
	procGetMessageW              = winUser32.NewProc("GetMessageW")
	procTranslateMessage         = winUser32.NewProc("TranslateMessage")
	procDispatchMessageW         = winUser32.NewProc("DispatchMessageW")
	procPostMessageW             = winUser32.NewProc("PostMessageW")
	procPostQuitMessage          = winUser32.NewProc("PostQuitMessage")
	procSendMessageW             = winUser32.NewProc("SendMessageW")
	procSetWindowPos             = winUser32.NewProc("SetWindowPos")
	procGetDpiForSystem          = winUser32.NewProc("GetDpiForSystem")
	procGetDpiForWindow          = winUser32.NewProc("GetDpiForWindow")
	procGetSystemMetrics         = winUser32.NewProc("GetSystemMetrics")
	procLoadCursorW              = winUser32.NewProc("LoadCursorW")
	procCreateIconFromResourceEx = winUser32.NewProc("CreateIconFromResourceEx")
	procMessageBoxW              = winUser32.NewProc("MessageBoxW")
	procDwmSetWindowAttribute    = winDwmapi.NewProc("DwmSetWindowAttribute")
	procCreateSolidBrush         = windows.NewLazySystemDLL("gdi32.dll").NewProc("CreateSolidBrush")
	procShellExecuteW            = winShell32.NewProc("ShellExecuteW")
	procSHCreateItemFromParsing  = winShell32.NewProc("SHCreateItemFromParsingName")
	procCoInitializeEx           = winOle32.NewProc("CoInitializeEx")
	procSetLastError             = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetLastError")
	procCoCreateInstance         = winOle32.NewProc("CoCreateInstance")
)

const (
	wmDestroy       = 0x0002
	wmMove          = 0x0003
	wmSize          = 0x0005
	wmActivate      = 0x0006
	wmClose         = 0x0010
	wmGetMinMaxInfo = 0x0024
	wmSetIcon       = 0x0080
	wmDpiChanged    = 0x02E0
	wmApp           = 0x8000
	wmDispatch      = wmApp + 1

	swHide    = 0
	swShow    = 5
	swRestore = 9

	wsOverlappedWindow = 0x00CF0000
	cwUseDefault       = 0x80000000
	idcArrow           = 32512
)

type winPoint struct{ X, Y int32 }

type winRect struct{ Left, Top, Right, Bottom int32 }

type winMsg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      winPoint
	Private uint32
}

type winClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type winMinMaxInfo struct {
	Reserved, MaxSize, MaxPosition, MinTrackSize, MaxTrackSize winPoint
}

const hostClassName = "AWLauncherWindow"

type hostWindow struct {
	hwnd    uintptr
	handler func(hwnd uintptr, msg uint32, wp, lp uintptr) (uintptr, bool)
	minW    int32
	minH    int32
}

var hostWindows = map[uintptr]*hostWindow{}
var creatingHost *hostWindow

var hostWndProc = windows.NewCallback(func(hwnd, msg, wp, lp uintptr) uintptr {
	w := hostWindows[hwnd]
	if w == nil && creatingHost != nil {
		w = creatingHost
		w.hwnd = hwnd
		hostWindows[hwnd] = w
	}
	if w != nil {
		switch uint32(msg) {
		case wmGetMinMaxInfo:
			info := *(**winMinMaxInfo)(unsafe.Pointer(&lp))
			dpi := windowDPI(hwnd)
			info.MinTrackSize = winPoint{scaleDPI(w.minW, dpi), scaleDPI(w.minH, dpi)}
			return 0
		case wmDpiChanged:
			r := *(**winRect)(unsafe.Pointer(&lp))
			procSetWindowPos.Call(hwnd, 0, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top), 0x0014)
			if w.handler != nil {
				w.handler(hwnd, uint32(msg), wp, lp)
			}
			return 0
		}
		if w.handler != nil {
			if r, handled := w.handler(hwnd, uint32(msg), wp, lp); handled {
				return r
			}
		}
		if uint32(msg) == wmDestroy {
			delete(hostWindows, hwnd)
		}
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
})

func createHostWindow(class, title string, owner uintptr, width, height, minW, minH int32, background uint32, handler func(uintptr, uint32, uintptr, uintptr) (uintptr, bool)) (*hostWindow, error) {
	var instance windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &instance); err != nil {
		return nil, err
	}
	className, _ := windows.UTF16PtrFromString(class)
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	brush, _, _ := procCreateSolidBrush.Call(uintptr(background))
	wc := winClassEx{
		Size:       uint32(unsafe.Sizeof(winClassEx{})),
		WndProc:    hostWndProc,
		Instance:   instance,
		Cursor:     windows.Handle(cursor),
		Background: windows.Handle(brush),
		ClassName:  className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	dpi, _, _ := procGetDpiForSystem.Call()
	if dpi == 0 {
		dpi = 96
	}
	w, h := scaleDPI(width, uint32(dpi)), scaleDPI(height, uint32(dpi))
	screenW, _, _ := procGetSystemMetrics.Call(0)
	screenH, _, _ := procGetSystemMetrics.Call(1)
	x, y := uintptr(cwUseDefault), uintptr(cwUseDefault)
	if int32(screenW) > w && int32(screenH) > h {
		x, y = uintptr((int32(screenW)-w)/2), uintptr((int32(screenH)-h)/2)
	}

	host := &hostWindow{handler: handler, minW: minW, minH: minH}
	creatingHost = host
	name, _ := windows.UTF16PtrFromString(title)
	hwnd, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(name)),
		wsOverlappedWindow, x, y, uintptr(w), uintptr(h), owner, 0, uintptr(instance), 0)
	creatingHost = nil
	if hwnd == 0 {
		return nil, err
	}
	host.hwnd = hwnd
	hostWindows[hwnd] = host
	return host, nil
}

func runMessageLoop() {
	var msg winMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func (w *hostWindow) show() { forceForeground(w.hwnd) }

var (
	procGetForegroundWindow = winUser32.NewProc("GetForegroundWindow")
	procAttachThreadInput   = winUser32.NewProc("AttachThreadInput")
	procBringWindowToTop    = winUser32.NewProc("BringWindowToTop")
	procGetCurrentThreadId  = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetCurrentThreadId")
	procFlashWindow         = winUser32.NewProc("FlashWindow")
)

func forceForeground(hwnd uintptr) {
	if iconic, _, _ := procIsIconic.Call(hwnd); iconic != 0 {
		procShowWindow.Call(hwnd, swRestore)
	} else {
		procShowWindow.Call(hwnd, swShow)
	}
	foreground, _, _ := procGetForegroundWindow.Call()
	if foreground == hwnd {
		return
	}
	ours, _, _ := procGetCurrentThreadId.Call()
	theirs, _, _ := procGetWindowThreadProcessId.Call(foreground, 0)
	if theirs != 0 && theirs != ours {
		procAttachThreadInput.Call(theirs, ours, 1)
		defer procAttachThreadInput.Call(theirs, ours, 0)
	}
	const flags = 0x0001 | 0x0002 | 0x0040
	procSetWindowPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, flags)
	procSetWindowPos.Call(hwnd, ^uintptr(1), 0, 0, 0, 0, flags)
	procBringWindowToTop.Call(hwnd)
	if ok, _, _ := procSetForegroundWindow.Call(hwnd); ok == 0 {
		procFlashWindow.Call(hwnd, 1)
	}
}

func (w *hostWindow) hide() { procShowWindow.Call(w.hwnd, swHide) }

func (w *hostWindow) visible() bool {
	r, _, _ := procIsWindowVisible.Call(w.hwnd)
	return r != 0
}

func (w *hostWindow) iconic() bool {
	r, _, _ := procIsIconic.Call(w.hwnd)
	return r != 0
}

func (w *hostWindow) post(msg uint32) { procPostMessageW.Call(w.hwnd, uintptr(msg), 0, 0) }

func (w *hostWindow) destroy() { procDestroyWindow.Call(w.hwnd) }

func (w *hostWindow) setIcon(small, big []byte) {
	for kind, image := range [][]byte{small, big} {
		if len(image) == 0 {
			continue
		}
		icon, _, _ := procCreateIconFromResourceEx.Call(uintptr(unsafe.Pointer(&image[0])), uintptr(len(image)), 1, 0x00030000, 0, 0, 0)
		if icon != 0 {
			procSendMessageW.Call(w.hwnd, wmSetIcon, uintptr(kind), icon)
		}
	}
}

func (w *hostWindow) iconSizes() (int, int) {
	dpi := windowDPI(w.hwnd)
	return int(scaleDPI(16, dpi)), int(scaleDPI(32, dpi))
}

func (w *hostWindow) setTitleBar(dark bool, caption, text uint32) {
	value := uint32(0)
	if dark {
		value = 1
	}
	procDwmSetWindowAttribute.Call(w.hwnd, 20, uintptr(unsafe.Pointer(&value)), 4)
	procDwmSetWindowAttribute.Call(w.hwnd, 35, uintptr(unsafe.Pointer(&caption)), 4)
	procDwmSetWindowAttribute.Call(w.hwnd, 36, uintptr(unsafe.Pointer(&text)), 4)
}

func windowDPI(hwnd uintptr) uint32 {
	if procGetDpiForWindow.Find() == nil {
		if dpi, _, _ := procGetDpiForWindow.Call(hwnd); dpi != 0 {
			return uint32(dpi)
		}
	}
	return 96
}

func scaleDPI(v int32, dpi uint32) int32 { return int32(int64(v) * int64(dpi) / 96) }

func messageBox(hwnd uintptr, title, text string, flags uint32) int {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(text)
	r, _, _ := procMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), uintptr(flags))
	return int(r)
}

func openInShell(target string) {
	verb, _ := windows.UTF16PtrFromString("open")
	path, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return
	}
	procShellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(path)), 0, 0, 1)
}

func clearLastError() { procSetLastError.Call(0) }

func comInit() { procCoInitializeEx.Call(0, 2) }

var (
	clsidFileOpenDialog = windows.GUID{Data1: 0xDC1C5A9C, Data2: 0xE88A, Data3: 0x4DDE, Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidFileOpenDialog   = windows.GUID{Data1: 0xD57C7288, Data2: 0xD4AD, Data3: 0x4768, Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
	iidShellItem        = windows.GUID{Data1: 0x43826D1E, Data2: 0xE718, Data3: 0x42EE, Data4: [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}
)

type comObject struct{ vtbl *[32]uintptr }

func (o *comObject) call(index int, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(o.vtbl[index], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return r
}

func (o *comObject) release() { o.call(2) }

var errPickerCancelled = errors.New("cancelled")

func pickFolder(owner uintptr, title, initial string) (string, error) {
	var dialog *comObject
	if hr, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidFileOpenDialog)), 0, 1,
		uintptr(unsafe.Pointer(&iidFileOpenDialog)), uintptr(unsafe.Pointer(&dialog))); int32(hr) < 0 {
		return "", syscall.Errno(hr)
	}
	defer dialog.release()
	var options uint32
	dialog.call(10, uintptr(unsafe.Pointer(&options)))
	dialog.call(9, uintptr(options|0x20|0x40|0x800))
	if t, err := windows.UTF16PtrFromString(title); err == nil {
		dialog.call(17, uintptr(unsafe.Pointer(t)))
	}
	if initial != "" {
		if p, err := windows.UTF16PtrFromString(initial); err == nil {
			var folder *comObject
			if hr, _, _ := procSHCreateItemFromParsing.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&iidShellItem)), uintptr(unsafe.Pointer(&folder))); int32(hr) >= 0 && folder != nil {
				dialog.call(12, uintptr(unsafe.Pointer(folder)))
				folder.release()
			}
		}
	}
	if hr := dialog.call(3, owner); int32(hr) < 0 {
		if uint32(hr) == 0x800704C7 {
			return "", errPickerCancelled
		}
		return "", syscall.Errno(hr)
	}
	var item *comObject
	if hr := dialog.call(20, uintptr(unsafe.Pointer(&item))); int32(hr) < 0 || item == nil {
		return "", syscall.Errno(hr)
	}
	defer item.release()
	var name *uint16
	if hr := item.call(5, 0x80058000, uintptr(unsafe.Pointer(&name))); int32(hr) < 0 || name == nil {
		return "", syscall.Errno(hr)
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(name))
	return windows.UTF16PtrToString(name), nil
}
