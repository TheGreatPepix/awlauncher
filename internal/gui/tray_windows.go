package gui

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"fyne.io/systray"
	"github.com/TheGreatPepix/awlauncher/internal/gui/appicon"
	"github.com/TheGreatPepix/awlauncher/internal/gui/ui"
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

func launcherTrayIcon() ([]byte, error) { return appicon.ICO(), nil }
