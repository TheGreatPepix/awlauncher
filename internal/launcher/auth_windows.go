package launcher

import (
	"fmt"
	"syscall"
	"unsafe"
)

type dataBlob struct {
	size uint32
	data *byte
}

var (
	crypt32        = syscall.NewLazyDLL("crypt32.dll")
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	cryptProtect   = crypt32.NewProc("CryptProtectData")
	cryptUnprotect = crypt32.NewProc("CryptUnprotectData")
	localFree      = kernel32.NewProc("LocalFree")

	getConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
)

func userCrypt(input []byte, protect bool) ([]byte, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("empty credentials")
	}
	in := dataBlob{size: uint32(len(input)), data: &input[0]}
	var out dataBlob
	proc := cryptUnprotect
	if protect {
		proc = cryptProtect
	}
	var ok uintptr
	var callErr error
	if protect {
		ok, _, callErr = proc.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&out)))
	} else {
		ok, _, callErr = proc.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&out)))
	}
	if ok == 0 {
		return nil, fmt.Errorf("Windows DPAPI: %w", callErr)
	}
	defer localFree.Call(uintptr(unsafe.Pointer(out.data)))
	return append([]byte(nil), unsafe.Slice(out.data, out.size)...), nil
}

func ownsConsole() bool {
	var pids [2]uint32
	n, _, _ := getConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), 2)
	return n == 1
}

func protectForUser(data []byte) ([]byte, error)   { return userCrypt(data, true) }
func unprotectForUser(data []byte) ([]byte, error) { return userCrypt(data, false) }
