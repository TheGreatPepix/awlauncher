package launcher

import (
	"fmt"
	"strings"
	"unicode/utf16"
	"unsafe"
)

var (
	openClipboard    = winUser32.NewProc("OpenClipboard")
	emptyClipboard   = winUser32.NewProc("EmptyClipboard")
	setClipboardData = winUser32.NewProc("SetClipboardData")
	closeClipboard   = winUser32.NewProc("CloseClipboard")
	globalAlloc      = kernel32.NewProc("GlobalAlloc")
	globalLock       = kernel32.NewProc("GlobalLock")
	globalUnlock     = kernel32.NewProc("GlobalUnlock")
	globalFree       = kernel32.NewProc("GlobalFree")
	copyUTF16        = kernel32.NewProc("lstrcpyW")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

func copyTextToClipboard(hwnd uintptr, value string) error {
	// CF_UNICODETEXT requires a movable, NUL-terminated UTF-16 allocation.
	value = strings.ReplaceAll(value, "\x00", "")
	value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\n", "\r\n")
	text := append(utf16.Encode([]rune(value)), 0)
	handle, _, err := globalAlloc.Call(gmemMoveable, uintptr(len(text)*2))
	if handle == 0 {
		return fmt.Errorf("GlobalAlloc: %w", err)
	}
	defer func() {
		if handle != 0 {
			globalFree.Call(handle)
		}
	}()
	ptr, _, err := globalLock.Call(handle)
	if ptr == 0 {
		return fmt.Errorf("GlobalLock: %w", err)
	}
	if copied, _, err := copyUTF16.Call(ptr, uintptr(unsafe.Pointer(&text[0]))); copied == 0 {
		globalUnlock.Call(handle)
		return fmt.Errorf("lstrcpyW: %w", err)
	}
	globalUnlock.Call(handle)
	opened, _, err := openClipboard.Call(hwnd)
	if opened == 0 {
		return fmt.Errorf("OpenClipboard: %w", err)
	}
	defer closeClipboard.Call()
	if ok, _, err := emptyClipboard.Call(); ok == 0 {
		return fmt.Errorf("EmptyClipboard: %w", err)
	}
	if ok, _, err := setClipboardData.Call(cfUnicodeText, handle); ok == 0 {
		return fmt.Errorf("SetClipboardData: %w", err)
	}
	handle = 0 // Windows owns the allocation after SetClipboardData succeeds.
	return nil
}
