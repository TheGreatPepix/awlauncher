package launcher

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestTrayIconLoadsInWindows(t *testing.T) {
	data, err := launcherTrayIcon()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "awlauncher.ico")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	const imageIcon = 1
	const loadFromFile = 0x10
	const defaultSize = 0x40
	icon, _, callErr := trayUser32.NewProc("LoadImageW").Call(0, uintptr(unsafe.Pointer(name)), imageIcon, 0, 0, loadFromFile|defaultSize)
	if icon == 0 {
		t.Fatalf("Windows could not load the tray icon: %v", callErr)
	}
	trayUser32.NewProc("DestroyIcon").Call(icon)
}

func TestTrayRegistersWithWindows(t *testing.T) {
	if os.Getenv("AWLAUNCHER_TEST_TRAY") != "1" {
		t.Skip("requires a Windows desktop session")
	}
	nothing := func() {}
	tray, err := startTray(trayActions{open: nothing, tap: nothing, exit: nothing, play: nothing})
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		text string
		on   bool
	}{{"Play Tanker · VK Play", true}, {"Play Tanker · VK Play", false}, {"", false}, {"Play EU main · FX ID, supertest", true}} {
		tray.setPlay(step.text, step.on)
		if tray.playText != step.text || tray.playOn != step.on {
			t.Fatalf("play item = %q %v, want %q %v", tray.playText, tray.playOn, step.text, step.on)
		}
	}
	tray.close()
	select {
	case <-tray.done:
	case <-time.After(5 * time.Second):
		t.Fatal("tray did not close")
	}
}
