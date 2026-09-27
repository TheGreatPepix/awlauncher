package launcher

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var getDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")

func fixedDrives() []string {
	var roots []string
	for letter := 'C'; letter <= 'Z'; letter++ {
		root := string(letter) + `:\`
		name, err := syscall.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		if kind, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(name))); kind == 3 {
			roots = append(roots, root)
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
	path, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var free uint64
	ok, _, callErr := getDiskFreeSpaceEx.Call(uintptr(unsafe.Pointer(path)), uintptr(unsafe.Pointer(&free)), 0, 0)
	if ok == 0 {
		return 0, callErr
	}
	return int64(free), nil
}
