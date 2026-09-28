package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/appicon"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/startup"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/ui"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/update"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
	"github.com/jchv/go-webview2/pkg/edge"
	"github.com/jchv/go-webview2/webviewloader"
)

const (
	windowTitle  = "AWLauncher"
	promptMarker = "\x1eawlauncher-prompt:"
	logLimit     = 200_000
	contextLines = 40
)

type guiApp struct {
	showMsg   uint32
	win       *hostWindow
	web       *edge.Chromium
	signIn    *signInState
	pageReady bool
	exiting   bool
	active    bool
	leftAt    time.Time
	updated   bool
	checked   bool
	inTray    bool

	queueMu sync.Mutex
	queue   []func()

	store   *config.Store
	session *session
	tray    *trayIcon
	gameUp  atomic.Bool

	opsMu  sync.Mutex
	ops    []operation
	nextOp int

	outMu   sync.Mutex
	logText []byte

	promptMu   sync.Mutex
	prompts    map[int]*pendingPrompt
	nextPrompt int
}

type pendingPrompt struct {
	kind     string
	question string
	def      bool
	op       string
	context  []string
	reply    chan promptReply
}

type promptReply struct {
	value string
	ok    bool
}

func RunGUI() int {
	runtime.LockOSThread()
	ui.SetLanguage(loadPrefs().Lang)
	updated := update.AfterUpdate(os.Args)
	inTray := startup.TrayRequested(os.Args)
	acquired, lockErr := acquireInstance()
	if lockErr != nil {
		messageBox(0, "AWLauncher", lockErr.Error(), 0x10)
		return 1
	}
	if !acquired {
		if !activateRunningGUI() {
			messageBox(0, "AWLauncher", ui.Translate("AWLauncher is already running in a console window. Close it first."), 0x30)
		}
		return 0
	}
	comInit()
	progress.UsePlainOutput()
	if version, err := webviewloader.GetInstalledVersion(); err != nil || version == "" {
		const text = "AWLauncher needs the Microsoft Edge WebView2 Runtime, which is part of Windows 11 and current Windows 10.\n\nOpen the download page?"
		if messageBox(0, "AWLauncher", ui.Translate(text), 0x04|0x10) == 6 {
			openInShell("https://go.microsoft.com/fwlink/p/?LinkId=2124703")
		}
		return 1
	}
	cfg, err := config.Load()
	if err != nil {
		messageBox(0, "AWLauncher", err.Error(), 0x10)
		return 1
	}
	g := &guiApp{store: config.NewStore(cfg), prompts: map[int]*pendingPrompt{}, showMsg: showWindowMessage(), updated: updated, inTray: inTray}
	if err := g.captureOutput(); err != nil {
		messageBox(0, "AWLauncher", ui.Translate("Cannot capture the launcher output: ")+err.Error(), 0x10)
		return 1
	}
	if err := g.createWindow(); err != nil {
		messageBox(0, "AWLauncher", err.Error(), 0x10)
		return 1
	}
	g.session = &session{cfg: g.store, client: authClient(), found: &foundGame{}}
	vkSignInStarted = g.vkSignInStarted
	bringLauncherForward = func() { g.post(g.showWindow) }
	tray, err := startTray(trayActions{
		open: func() { g.post(g.showWindow) },
		tap:  func() { g.post(g.toggleWindow) },
		exit: func() { g.post(g.exit) },
		play: func() { g.post(g.playLast) },
	})
	if err != nil {
		messageBox(g.win.hwnd, "AWLauncher", ui.Translate("Cannot create the tray icon: ")+err.Error(), 0x10)
		return 1
	}
	defer tray.close()
	g.tray = tray
	g.updateTray()
	startup.Refresh()
	go g.watchProgress()
	go g.watchGame()
	runMessageLoop()
	return 0
}

func (g *guiApp) createWindow() error {
	dark, surface, text := themeColors(loadPrefs().Theme)
	win, err := createHostWindow(hostClassName, windowTitle, 0, 1060, 720, 780, 560, colorRef(surface), g.wndProc)
	if err != nil {
		return fmt.Errorf("cannot create the window: %w", err)
	}
	g.win = win
	g.setWindowIcon()
	win.setTitleBar(dark, colorRef(surface), colorRef(text))
	if !g.inTray {
		g.showWindow()
	}

	web := edge.NewChromium()
	web.MessageCallback = g.onMessage
	if dir, err := platform.DataDir(); err == nil {
		web.DataPath = filepath.Join(dir, "WebView2")
	}
	if !web.Embed(win.hwnd) {
		return errors.New("cannot start Microsoft Edge WebView2")
	}
	g.web = web
	devtools := os.Getenv("AWLAUNCHER_DEVTOOLS") == "1"
	clearLastError()
	if settings, err := web.GetSettings(); err == nil {
		_ = settings.PutAreDevToolsEnabled(devtools)
		_ = settings.PutAreDefaultContextMenusEnabled(devtools)
		_ = settings.PutIsStatusBarEnabled(false)
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

func (g *guiApp) setWindowIcon() { setAppIcon(g.win) }

func setAppIcon(w *hostWindow) {
	small, big := w.iconSizes()
	s, err1 := appicon.Image(small)
	b, err2 := appicon.Image(big)
	if err1 == nil && err2 == nil {
		w.setIcon(s, b)
	}
}

func themeColors(theme string) (dark bool, surface, text [3]uint8) {
	if theme == "dark" || (theme != "light" && systemDarkTheme()) {
		return true, [3]uint8{0x14, 0x12, 0x18}, [3]uint8{0xE6, 0xE0, 0xE9}
	}
	return false, [3]uint8{0xFD, 0xF7, 0xFF}, [3]uint8{0x1D, 0x1B, 0x20}
}

func (g *guiApp) wndProc(_ uintptr, msg uint32, wp, _ uintptr) (uintptr, bool) {
	if msg == g.showMsg && msg != 0 {
		g.showWindow()
		return 0, true
	}
	switch msg {
	case wmSize:
		if g.web != nil {
			g.web.Resize()
		}
	case wmMove:
		if g.web != nil {
			_ = g.web.NotifyParentWindowPositionChanged()
		}
	case wmActivate:
		g.active = wp&0xFFFF != 0
		if !g.active {
			g.leftAt = time.Now()
		}
		if g.active && g.web != nil && g.pageReady {
			g.web.Focus()
		}
	case wmClose:
		if !g.exiting {
			g.hideWindow()
			return 0, true
		}
	case wmDestroy:
		procPostQuitMessage.Call(0)
	case wmDpiChanged:
		g.setWindowIcon()
	case wmDispatch:
		g.drain()
		return 0, true
	}
	return 0, false
}

func (g *guiApp) post(f func()) {
	g.queueMu.Lock()
	g.queue = append(g.queue, f)
	g.queueMu.Unlock()
	g.win.post(wmDispatch)
}

func (g *guiApp) drain() {
	g.queueMu.Lock()
	q := g.queue
	g.queue = nil
	g.queueMu.Unlock()
	for _, f := range q {
		f()
	}
}

func (g *guiApp) showWindow() {
	g.win.show()
	if g.web != nil {
		_ = g.web.Show()
		g.web.Resize()
	}
}

func (g *guiApp) toggleWindow() {
	inFront := g.active || time.Since(g.leftAt) < time.Second
	if g.win.visible() && !g.win.iconic() && inFront {
		g.hideWindow()
		return
	}
	g.showWindow()
}

func (g *guiApp) hideWindow() {
	g.win.hide()
	if g.web != nil {
		_ = g.web.Hide()
	}
}

func (g *guiApp) exit() {
	g.exiting = true
	g.win.destroy()
}

func (g *guiApp) emit(event any) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	g.post(func() {
		if g.pageReady {
			g.web.Eval("window.aw && aw.recv(" + string(data) + ")")
		}
	})
}
