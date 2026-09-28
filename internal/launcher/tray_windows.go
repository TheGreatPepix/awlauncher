package launcher

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/ui"

	"fyne.io/systray"
	"github.com/TheGreatPepix/awlauncher/internal/appicon"
)

var (
	trayUser32           = syscall.NewLazyDLL("user32.dll")
	trayGetConsoleWindow = kernel32.NewProc("GetConsoleWindow")
	trayShowWindow       = trayUser32.NewProc("ShowWindow")
	trayIsWindowVisible  = trayUser32.NewProc("IsWindowVisible")
	trayIsIconic         = trayUser32.NewProc("IsIconic")
	traySetForeground    = trayUser32.NewProc("SetForegroundWindow")
)

type trayIcon struct {
	ready chan struct{}
	done  chan struct{}

	mu       sync.Mutex
	openItem *systray.MenuItem
	exitItem *systray.MenuItem
	playItem *systray.MenuItem
	playText string
	playOn   bool
}

type trayActions struct {
	open, tap, exit, play func()
}

func startTray(a trayActions) (*trayIcon, error) {
	t := &trayIcon{ready: make(chan struct{}), done: make(chan struct{})}
	icon, err := launcherTrayIcon()
	if err != nil {
		return nil, err
	}
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(t.done)
		systray.Run(func() {
			systray.SetIcon(icon)
			systray.SetTooltip("AWLauncher")
			systray.SetOnTapped(a.tap)
			var played chan struct{}
			if a.play != nil {
				t.playItem = systray.AddMenuItem(ui.Translate("Play"), ui.Translate("Start the last played account"))
				t.playItem.Hide()
				played = t.playItem.ClickedCh
			}
			openItem := systray.AddMenuItem(ui.Translate("Open AWLauncher"), ui.Translate("Show the launcher"))
			systray.AddSeparator()
			exitItem := systray.AddMenuItem(ui.Translate("Exit"), ui.Translate("Close the launcher"))
			t.openItem, t.exitItem = openItem, exitItem
			go func() {
				for {
					select {
					case <-played:
						a.play()
					case <-openItem.ClickedCh:
						a.open()
					case <-exitItem.ClickedCh:
						a.exit()
					case <-t.done:
						return
					}
				}
			}()
			close(t.ready)
		}, nil)
	}()
	select {
	case <-t.ready:
		return t, nil
	case <-t.done:
		return nil, fmt.Errorf("the system tray is unavailable")
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("the system tray did not start")
	}
}

func (t *trayIcon) relabel() {
	if t == nil || t.openItem == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.openItem.SetTitle(ui.Translate("Open AWLauncher"))
	t.openItem.SetTooltip(ui.Translate("Show the launcher"))
	t.exitItem.SetTitle(ui.Translate("Exit"))
	t.exitItem.SetTooltip(ui.Translate("Close the launcher"))
	if t.playItem != nil {
		t.playItem.SetTooltip(ui.Translate("Start the last played account"))
	}
}

func (t *trayIcon) setPlay(text string, enabled bool) {
	if t == nil || t.playItem == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if text == t.playText && enabled == t.playOn {
		return
	}
	if text == "" {
		t.playItem.Hide()
	} else {
		t.playItem.SetTitle(text)
		if enabled {
			t.playItem.Enable()
		} else {
			t.playItem.Disable()
		}
		if t.playText == "" {
			t.playItem.Show()
		}
	}
	t.playText, t.playOn = text, enabled
}

func (t *trayIcon) close() {
	if t == nil {
		return
	}
	select {
	case <-t.done:
		return
	default:
		systray.Quit()
	}
}

type trayController struct {
	*trayIcon
	restore chan struct{}
	hidden  atomic.Bool
}

func newTrayController() (*trayController, error) {
	t := &trayController{restore: make(chan struct{}, 1)}
	open := func() {
		showLauncherConsole()
		if t.hidden.Load() {
			select {
			case t.restore <- struct{}{}:
			default:
			}
		}
	}
	tap := func() {
		if !t.hidden.Load() && consoleShown() {
			showLauncherWindow(false)
			return
		}
		open()
	}
	icon, err := startTray(trayActions{open: open, tap: tap, exit: func() {
		systray.Quit()
		os.Exit(0)
	}})
	if err != nil {
		return nil, err
	}
	t.trayIcon = icon
	return t, nil
}

func (t *trayController) hideUntilRestored() bool {
	t.hidden.Store(true)
	showLauncherWindow(false)
	select {
	case <-t.restore:
		t.hidden.Store(false)
		showLauncherConsole()
		return true
	case <-t.done:
		t.hidden.Store(false)
		showLauncherConsole()
		return false
	}
}

func showLauncherWindow(show bool) {
	hwnd, _, _ := trayGetConsoleWindow.Call()
	if hwnd == 0 {
		return
	}
	command := uintptr(0)
	if show {
		command = 9
	}
	trayShowWindow.Call(hwnd, command)
	if show {
		traySetForeground.Call(hwnd)
	}
}

func showLauncherConsole() { showLauncherWindow(true) }

func consoleShown() bool {
	hwnd, _, _ := trayGetConsoleWindow.Call()
	if hwnd == 0 {
		return false
	}
	visible, _, _ := trayIsWindowVisible.Call(hwnd)
	iconic, _, _ := trayIsIconic.Call(hwnd)
	return visible != 0 && iconic == 0
}

func launcherTrayIcon() ([]byte, error) { return appicon.ICO(), nil }
