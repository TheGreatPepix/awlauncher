package gui

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/gui/appicon"
	"github.com/TheGreatPepix/awlauncher/internal/gui/startup"
	"github.com/TheGreatPepix/awlauncher/internal/gui/ui"
	"github.com/TheGreatPepix/awlauncher/internal/gui/update"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
	"github.com/jchv/go-webview2/pkg/edge"
	"github.com/jchv/go-webview2/webviewloader"
)

const windowTitle = "AWLauncher"

type winHost struct {
	app     *App
	showMsg uint32
	win     *hostWindow
	web     *edge.Chromium
	tray    *trayIcon
	inTray  bool
	active  bool
	leftAt  time.Time
	exiting bool

	queueMu sync.Mutex
	queue   []func()

	signWin    *hostWindow
	signWeb    *edge.Chromium
	signReport func(code, state string)
	signClosed func()
}

func Run() int {
	runtime.LockOSThread()
	ui.SetLanguage(loadPrefs().Lang)
	updated := update.AfterUpdate(os.Args)
	inTray := startup.TrayRequested(os.Args)
	acquired, lockErr := acquireInstance()
	if lockErr != nil {
		messageBox(0, windowTitle, lockErr.Error(), 0x10)
		return 1
	}
	if !acquired {
		if !activateRunningGUI() {
			messageBox(0, windowTitle, ui.Translate("AWLauncher is already running."), 0x30)
		}
		return 0
	}
	comInit()
	if version, err := webviewloader.GetInstalledVersion(); err != nil || version == "" {
		const text = "AWLauncher needs the Microsoft Edge WebView2 Runtime, which is part of Windows 11 and current Windows 10.\n\nOpen the download page?"
		if messageBox(0, windowTitle, ui.Translate(text), 0x04|0x10) == 6 {
			openInShell("https://go.microsoft.com/fwlink/p/?LinkId=2124703")
		}
		return 1
	}
	logs := &logBuffer{}
	log.SetFlags(0)
	log.SetOutput(logs)
	cfg, err := config.Load()
	if err != nil {
		messageBox(0, windowTitle, err.Error(), 0x10)
		return 1
	}
	g := newApp(config.NewStore(cfg), logs, updated)
	h := &winHost{app: g, showMsg: showWindowMessage(), inTray: inTray}
	g.host = h
	if err := h.createWindow(); err != nil {
		messageBox(0, windowTitle, err.Error(), 0x10)
		return 1
	}
	logs.follow(g.emitLog)
	tray, err := startTray(trayActions{
		open: func() { h.post(h.showWindow) },
		tap:  func() { h.post(h.toggleWindow) },
		exit: func() { h.post(g.requestExit) },
		play: func() { h.post(g.playLast) },
	})
	if err != nil {
		messageBox(h.win.hwnd, windowTitle, ui.Translate("Cannot create the tray icon: ")+err.Error(), 0x10)
		return 1
	}
	defer tray.close()
	h.tray = tray
	g.updateTray()
	startup.Refresh()
	go g.watchProgress()
	go g.watchGame()
	runMessageLoop()
	return 0
}

func (h *winHost) createWindow() error {
	dark, surface, text := themeColors(loadPrefs().Theme)
	win, err := createHostWindow(hostClassName, windowTitle, 0, 1060, 720, 780, 560, colorRef(surface), h.wndProc)
	if err != nil {
		return fmt.Errorf("cannot create the window: %w", err)
	}
	h.win = win
	setAppIcon(win)
	win.setTitleBar(dark, colorRef(surface), colorRef(text))
	if !h.inTray {
		h.showWindow()
	}
	web, err := newWebView(win)
	if err != nil {
		return err
	}
	web.MessageCallback = h.app.onMessage
	h.web = web
	devtools := os.Getenv("AWLAUNCHER_DEVTOOLS") == "1"
	if settings, err := web.GetSettings(); err == nil {
		_ = settings.PutAreDefaultContextMenusEnabled(devtools)
		_ = settings.PutIsZoomControlEnabled(false)
		_ = settings.PutAreBrowserAcceleratorKeysEnabled(devtools)
	}
	if c2 := web.GetController().GetICoreWebView2Controller2(); c2 != nil {
		_ = c2.PutDefaultBackgroundColor(edge.COREWEBVIEW2_COLOR{A: 255, R: surface[0], G: surface[1], B: surface[2]})
	}
	page, err := ui.Page()
	if err != nil {
		return err
	}
	web.Resize()
	web.NavigateToString(page)
	return nil
}

func newWebView(win *hostWindow) (*edge.Chromium, error) {
	web := edge.NewChromium()
	if dir, err := platform.DataDir(); err == nil {
		web.DataPath = filepath.Join(dir, "WebView2")
	}
	if !web.Embed(win.hwnd) {
		return nil, errors.New("cannot start Microsoft Edge WebView2")
	}
	clearLastError()
	if settings, err := web.GetSettings(); err == nil {
		_ = settings.PutAreDevToolsEnabled(os.Getenv("AWLAUNCHER_DEVTOOLS") == "1")
		_ = settings.PutIsStatusBarEnabled(false)
	}
	return web, nil
}

func setAppIcon(w *hostWindow) {
	small, big := w.iconSizes()
	s, err1 := appicon.Image(small)
	b, err2 := appicon.Image(big)
	if err1 == nil && err2 == nil {
		w.setIcon(s, b)
	}
}

func (h *winHost) wndProc(_ uintptr, msg uint32, wp, _ uintptr) (uintptr, bool) {
	if msg == h.showMsg && msg != 0 {
		h.showWindow()
		return 0, true
	}
	switch msg {
	case wmSize:
		if h.web != nil {
			h.web.Resize()
		}
	case wmMove:
		if h.web != nil {
			_ = h.web.NotifyParentWindowPositionChanged()
		}
	case wmActivate:
		h.active = wp&0xFFFF != 0
		if !h.active {
			h.leftAt = time.Now()
		}
		if h.active && h.web != nil && h.app.pageReady.Load() {
			h.web.Focus()
		}
	case wmClose:
		if !h.exiting {
			h.hideWindow()
			return 0, true
		}
	case wmDestroy:
		procPostQuitMessage.Call(0)
	case wmDpiChanged:
		setAppIcon(h.win)
	case wmDispatch:
		h.drain()
		return 0, true
	}
	return 0, false
}

func (h *winHost) post(f func()) {
	h.queueMu.Lock()
	h.queue = append(h.queue, f)
	h.queueMu.Unlock()
	h.win.post(wmDispatch)
}

func (h *winHost) drain() {
	h.queueMu.Lock()
	q := h.queue
	h.queue = nil
	h.queueMu.Unlock()
	for _, f := range q {
		f()
	}
}

func (h *winHost) eval(script string) { h.web.Eval(script) }

func (h *winHost) showWindow() {
	h.win.show()
	if h.web != nil {
		_ = h.web.Show()
		h.web.Resize()
	}
}

func (h *winHost) toggleWindow() {
	inFront := h.active || time.Since(h.leftAt) < time.Second
	if h.win.visible() && !h.win.iconic() && inFront {
		h.hideWindow()
		return
	}
	h.showWindow()
}

func (h *winHost) hideWindow() {
	h.win.hide()
	if h.web != nil {
		_ = h.web.Hide()
	}
}

func (h *winHost) exit() {
	h.exiting = true
	time.AfterFunc(10*time.Second, func() { os.Exit(0) })
	h.win.destroy()
}

func (h *winHost) setTitleBar(dark bool, caption, text [3]uint8) {
	h.win.setTitleBar(dark, colorRef(caption), colorRef(text))
}

func (h *winHost) pickFolder(initial string) (string, error) {
	return pickFolder(h.win.hwnd, "Game folder", initial)
}

func (h *winHost) copyText(text string) error { return copyTextToClipboard(h.win.hwnd, text) }

func (h *winHost) open(target string) { openInShell(target) }

func (h *winHost) setTrayPlay(text string, enabled bool) { h.tray.setPlay(text, enabled) }

func (h *winHost) relabelTray() { h.tray.relabel() }
