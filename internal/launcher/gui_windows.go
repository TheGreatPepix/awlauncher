package launcher

import (
	"bufio"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/appicon"
	"github.com/TheGreatPepix/awlauncher/internal/fileutil"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
	"github.com/jchv/go-webview2/pkg/edge"
	"github.com/jchv/go-webview2/webviewloader"
	"golang.org/x/sys/windows/registry"
)

//go:embed ui
var uiAssets embed.FS

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
	shown     bool
	exiting   bool
	active    bool
	leftAt    time.Time
	updated   bool
	checked   bool
	inTray    bool

	queueMu sync.Mutex
	queue   []func()

	store   *configStore
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
	setGUILang(loadPrefs().Lang)
	updated := afterUpdate(os.Args)
	inTray := hasArg(os.Args, trayFlag)
	if !acquireInstance() {
		if !activateRunningGUI() {
			messageBox(0, "AWLauncher", tr("AWLauncher is already running in a console window. Close it first."), 0x30)
		}
		return 0
	}
	comInit()
	progress.UsePlainOutput()
	if version, err := webviewloader.GetInstalledVersion(); err != nil || version == "" {
		const text = "AWLauncher needs the Microsoft Edge WebView2 Runtime, which is part of Windows 11 and current Windows 10.\n\nOpen the download page?"
		if messageBox(0, "AWLauncher", tr(text), 0x04|0x10) == 6 {
			openInShell("https://go.microsoft.com/fwlink/p/?LinkId=2124703")
		}
		return 1
	}
	cfg, err := loadConfig()
	if err != nil {
		messageBox(0, "AWLauncher", err.Error(), 0x10)
		return 1
	}
	g := &guiApp{store: newConfigStore(cfg), prompts: map[int]*pendingPrompt{}, showMsg: showWindowMessage(), updated: updated, inTray: inTray}
	if err := g.captureOutput(); err != nil {
		messageBox(0, "AWLauncher", tr("Cannot capture the launcher output: ")+err.Error(), 0x10)
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
		messageBox(g.win.hwnd, "AWLauncher", tr("Cannot create the tray icon: ")+err.Error(), 0x10)
		return 1
	}
	defer tray.close()
	g.tray = tray
	g.updateTray()
	refreshAutostart()
	go g.watchProgress()
	go g.watchGame()
	runMessageLoop()
	return 0
}

func (g *guiApp) createWindow() error {
	dark, surface, text := themeColors()
	win, err := createHostWindow(hostClassName, windowTitle, 0, 1060, 720, 780, 560, colorRef(surface), g.wndProc)
	if err != nil {
		return fmt.Errorf("cannot create the window: %w", err)
	}
	g.win = win
	g.setWindowIcon()
	win.setTitleBar(dark, colorRef(surface), colorRef(text))

	web := edge.NewChromium()
	web.MessageCallback = g.onMessage
	if dir, err := launcherDir(); err == nil {
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
	page, err := uiPage()
	if err != nil {
		return err
	}
	web.Resize()
	web.NavigateToString(page)
	time.AfterFunc(5*time.Second, func() { g.post(func() { g.reveal() }) })
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

func themeColors() (dark bool, surface, text [3]uint8) {
	if systemDarkTheme() {
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

func (g *guiApp) reveal() {
	if !g.shown && !g.inTray {
		g.showWindow()
	}
}

func (g *guiApp) showWindow() {
	g.shown = true
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

func uiPage() (string, error) {
	html, err := uiAssets.ReadFile("ui/index.html")
	if err != nil {
		return "", err
	}
	css, err := uiAssets.ReadFile("ui/app.css")
	if err != nil {
		return "", err
	}
	js, err := uiAssets.ReadFile("ui/app.js")
	if err != nil {
		return "", err
	}
	i18nJS, err := uiAssets.ReadFile("ui/i18n.js")
	if err != nil {
		return "", err
	}
	font, err := uiAssets.ReadFile("ui/fonts/Rubik-Variable.ttf")
	if err != nil {
		return "", err
	}
	style := strings.Replace(string(css), `url("fonts/Rubik-Variable.ttf")`, `url("data:font/ttf;base64,`+base64.StdEncoding.EncodeToString(font)+`")`, 1)
	page := strings.Replace(string(html), `<link rel="stylesheet" href="app.css">`, "<style>\n"+style+"</style>", 1)
	page = strings.Replace(page, `<script src="i18n.js"></script>`, "<script>\n"+string(i18nJS)+"</script>", 1)
	page = strings.Replace(page, `<script src="app.js"></script>`, "<script>\n"+string(js)+"</script>", 1)
	if strings.Contains(page, `href="app.css"`) || strings.Contains(page, `src="app.js"`) || strings.Contains(page, `src="i18n.js"`) || strings.Contains(page, `url("fonts/`) {
		return "", errors.New("the embedded page references files that were not inlined")
	}
	return page, nil
}

type uiPrefs struct {
	Theme string `json:"theme"`
	Hue   int    `json:"hue"`
	Lang  string `json:"lang"`
}

func prefsPath() (string, error) {
	dir, err := launcherDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ui.json"), nil
}

func loadPrefs() uiPrefs {
	p := uiPrefs{Theme: "system", Hue: 300, Lang: "auto"}
	if path, err := prefsPath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, &p)
		}
	}
	return p
}

func savePrefs(p uiPrefs) error {
	path, err := prefsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.WriteAtomic(path, data)
}

type uiCommand struct {
	Cmd      string `json:"cmd"`
	Account  string `json:"account"`
	Provider string `json:"provider"`
	Prompt   int    `json:"prompt"`
	Value    string `json:"value"`
	OK       bool   `json:"ok"`
	Dark     bool   `json:"dark"`
	Caption  string `json:"caption"`
	Text     string `json:"text"`
	Theme    string `json:"theme"`
	Branch   string `json:"branch"`
	Hue      int    `json:"hue"`
	Lang     string `json:"lang"`
}

func (g *guiApp) onMessage(message string) {
	var c uiCommand
	if err := json.Unmarshal([]byte(message), &c); err != nil {
		return
	}
	switch c.Cmd {
	case "ready":
		g.pageReady = true
		g.emitState(true)
		g.post(g.reveal)
		if g.updated {
			g.updated = false
			fmt.Println("AWLauncher is updated to", Version+".")
			g.notice("AWLauncher is updated to " + Version)
		}
		if !g.checked && releaseBuild(Version) {
			g.checked = true
			go func() {
				info := checkForUpdate(authClient())
				info.Quiet = true
				g.emit(info)
			}()
		}
	case "pause":
		if progress.Default.Pause() {
			fmt.Println("Paused. Press Resume to go on from where it stopped.")
		}
	case "resume":
		if progress.Default.Resume() {
			fmt.Println("Resumed.")
		}
	case "autostart":
		if err := setAutostart(c.Value); err != nil {
			g.notice("Cannot change the start with Windows: " + err.Error())
		}
		g.emitState(false)
	case "checkUpdate":
		go func() { g.emit(checkForUpdate(authClient())) }()
	case "applyUpdate":
		g.updateSelf()
	case "theme":
		caption, ok1 := parseHexColor(c.Caption)
		text, ok2 := parseHexColor(c.Text)
		if ok1 && ok2 {
			g.win.setTitleBar(c.Dark, colorRef(caption), colorRef(text))
		}
	case "prefs":
		if err := savePrefs(uiPrefs{Theme: c.Theme, Hue: c.Hue, Lang: c.Lang}); err != nil {
			fmt.Println("Cannot save the appearance settings:", err)
		}
		if setGUILang(c.Lang) && g.tray != nil {
			g.tray.relabel()
			g.updateTray()
		}
	case "answer":
		g.answer(c.Prompt, c.Value, c.OK)
	case "browse":
		id, initial := c.Prompt, c.Value
		g.post(func() {
			dir, err := pickFolder(g.win.hwnd, "Game folder", initial)
			if err == nil {
				g.emit(map[string]any{"type": "browsed", "prompt": id, "path": dir})
			}
		})
	case "folderInfo":
		id, dir := c.Prompt, c.Value
		go func() { g.emit(describeFolder(id, dir)) }()
	case "gameInfo":
		go func() { g.emit(describeGame(g.store.get().Game)) }()
	case "verify", "removeBranch":
		g.clientCommand(c.Cmd, c.Value, c.Branch)
	case "clearDownloads":
		g.run(operation{Title: "Deleting downloaded patches", Game: true}, func(s *session) (bool, error) {
			return false, s.clearDownloads()
		})
	case "uninstall":
		g.run(operation{Title: "Uninstalling the game", Game: true}, func(s *session) (bool, error) {
			return false, s.uninstallGame()
		})
	case "gameFolder":
		g.post(g.changeGameFolder)
	case "openGameFolder":
		if game := g.store.get().Game; game != "" {
			openInShell(game)
		}
	case "openDataFolder":
		if dir, err := launcherDir(); err == nil {
			openInShell(dir)
		}
	case "openLink":
		if strings.HasPrefix(c.Value, "https://") {
			openInShell(c.Value)
		}
	case "add":
		g.addAccount(c.Provider)
	case "signinOpen", "signinFresh", "signinHere", "signinCancel":
		cmd := c.Cmd
		g.post(func() { g.signInCommand(cmd) })
	case "rename":
		g.rename(c.Account, c.Value)
	case "closeGame":
		go func() {
			fmt.Println("Closing the game...")
			if err := closeGame(10 * time.Second); err != nil {
				fmt.Println("Error:", err)
				g.emit(map[string]any{"type": "notice", "message": err.Error()})
			} else {
				fmt.Println("The game is closed.")
				g.emit(map[string]any{"type": "notice", "message": "The game is closed"})
			}
			g.checkGame()
		}()
	case "play", "remove", "branches", "key", "pin":
		acc, ok := g.findAccount(c.Account)
		if !ok {
			return
		}
		switch c.Cmd {
		case "play":
			g.play(acc)
		case "remove":
			g.remove(acc)
		case "branches":
			g.showBranches(acc)
		case "key":
			g.activateKey(acc)
		case "pin":
			g.pin(acc)
		}
	}
}

type uiAccount struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Login    string `json:"login"`
	Service  string `json:"service"`
	Provider string `json:"provider"`
	Branch   string `json:"branch,omitempty"`
	Last     bool   `json:"last"`
}

type uiState struct {
	Type       string      `json:"type"`
	Accounts   []uiAccount `json:"accounts"`
	Game       string      `json:"game"`
	Data       string      `json:"data"`
	Version    string      `json:"version"`
	Autostart  string      `json:"autostart"`
	SystemLang string      `json:"systemLang"`
	Ops        []operation `json:"ops"`
	Running    bool        `json:"running"`
	Log        *string     `json:"log,omitempty"`
	Prefs      *uiPrefs    `json:"prefs,omitempty"`
}

func accountViews(cfg launcherConfig) []uiAccount {
	views := make([]uiAccount, 0, len(cfg.Accounts))
	for _, a := range cfg.Accounts {
		login, name := a.login(), a.displayName()
		provider := "vkplay"
		if a.isFX() {
			provider = "fxid"
		}
		views = append(views, uiAccount{
			ID: strconv.FormatInt(a.UserID, 10), Name: name, Login: login, Service: a.service(),
			Provider: provider, Branch: a.Branch, Last: a.UserID == cfg.LastUserID,
		})
	}
	return views
}

func (g *guiApp) emitState(withLog bool) {
	cfg := g.store.get()
	g.opsMu.Lock()
	ops := append([]operation{}, g.ops...)
	g.opsMu.Unlock()
	s := uiState{Type: "state", Accounts: accountViews(cfg), Game: cfg.Game, Ops: ops, Running: g.gameUp.Load(), Version: Version, Autostart: autostartMode(), SystemLang: uiLanguage()}
	if dir, err := launcherDir(); err == nil {
		s.Data = dir
	}
	if withLog {
		g.outMu.Lock()
		text := string(g.logText)
		g.outMu.Unlock()
		s.Log = &text
		prefs := loadPrefs()
		s.Prefs = &prefs
	}
	g.emit(s)
	g.updateTray()
}

func (g *guiApp) updateTray() {
	if g.tray == nil {
		return
	}
	acc, ok := g.store.find(g.store.get().LastUserID)
	if !ok {
		g.tray.setPlay("", false)
		return
	}
	g.tray.setPlay(tr("Play")+" "+acc.displayName()+" · "+acc.service(), !g.gameBusy() && !g.gameUp.Load())
}

func (g *guiApp) clientCommand(cmd, kind, branch string) {
	c, err := g.session.findClient(kind, branch)
	if err != nil {
		g.notice(err.Error())
		return
	}
	if cmd == "verify" {
		g.run(operation{Title: "Checking " + c.name(), Game: true}, func(s *session) (bool, error) {
			return false, s.verifyClient(c)
		})
		return
	}
	g.run(operation{Title: "Removing " + c.name(), Game: true}, func(s *session) (bool, error) {
		return false, s.removeBranch(c)
	})
}

func (g *guiApp) updateSelf() {
	g.opsMu.Lock()
	busy := ""
	if len(g.ops) > 0 {
		busy = g.ops[0].Title
	}
	g.opsMu.Unlock()
	if busy != "" {
		g.notice("Wait until “" + busy + "” finishes")
		return
	}
	g.run(operation{Title: "Updating AWLauncher", Game: true}, func(*session) (bool, error) {
		exe, err := installUpdate(&http.Client{Timeout: 10 * time.Minute})
		if err != nil {
			return false, err
		}
		g.post(func() {
			g.opsMu.Lock()
			others := len(g.ops) - 1
			g.opsMu.Unlock()
			if others > 0 {
				fmt.Println("The update takes effect when AWLauncher starts next time.")
				return
			}
			if err := startUpdated(exe); err != nil {
				fmt.Println("Cannot restart AWLauncher:", err)
				return
			}
			g.exit()
		})
		return false, nil
	})
}

func (g *guiApp) playLast() {
	if acc, ok := g.store.find(g.store.get().LastUserID); ok {
		g.play(acc)
	}
}

func (g *guiApp) findAccount(id string) (account, bool) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return account{}, false
	}
	return g.store.find(n)
}

type operation struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Game    bool   `json:"game,omitempty"`
	Account string `json:"account,omitempty"`
}

func (g *guiApp) begin(op *operation) string {
	g.opsMu.Lock()
	defer g.opsMu.Unlock()
	for _, o := range g.ops {
		if (op.Game && o.Game) || (op.Account != "" && op.Account == o.Account) {
			return "Wait until “" + o.Title + "” finishes"
		}
	}
	g.nextOp++
	op.ID = g.nextOp
	g.ops = append(g.ops, *op)
	return ""
}

func (g *guiApp) finish(id int) int {
	g.opsMu.Lock()
	defer g.opsMu.Unlock()
	for i, o := range g.ops {
		if o.ID == id {
			g.ops = append(g.ops[:i], g.ops[i+1:]...)
			break
		}
	}
	return len(g.ops)
}

func (g *guiApp) gameBusy() bool {
	g.opsMu.Lock()
	defer g.opsMu.Unlock()
	for _, o := range g.ops {
		if o.Game {
			return true
		}
	}
	return false
}

func (g *guiApp) notice(message string) {
	g.emit(map[string]any{"type": "notice", "message": message})
}

func (g *guiApp) run(op operation, action func(s *session) (bool, error)) {
	if reason := g.begin(&op); reason != "" {
		g.notice(reason)
		return
	}
	g.emitState(false)
	s := g.opSession(op)
	go func() {
		launched, err := action(s)
		g.post(func() {
			others := g.finish(op.ID)
			result := map[string]any{"type": "done", "id": op.ID, "title": op.Title}
			switch {
			case errors.Is(err, errQuit):
				result["status"] = "cancelled"
			case err != nil:
				result["status"] = "error"
				result["message"] = strings.TrimSpace(err.Error())
				fmt.Println("Error:", strings.TrimSpace(err.Error()))
				g.showWindow()
			case launched:
				result["status"] = "launched"
				if others == 0 {
					g.hideWindow()
				}
			default:
				result["status"] = "ok"
			}
			g.emitState(false)
			g.emit(result)
		})
	}()
}

func accountOp(title string, acc account) operation {
	return operation{Title: title, Account: strconv.FormatInt(acc.UserID, 10)}
}

func (g *guiApp) play(acc account) {
	op := accountOp("Starting "+acc.label(), acc)
	op.Game = true
	g.run(op, func(s *session) (bool, error) {
		err := s.play(acc)
		if errors.Is(err, errNeedLogin) {
			relogged, loginErr := s.relogin(acc)
			if loginErr != nil {
				return false, loginErr
			}
			acc = relogged
			err = s.play(acc)
		}
		if err != nil {
			return false, err
		}
		return true, g.store.update(func(c *launcherConfig) { c.LastUserID = acc.UserID })
	})
}

func (g *guiApp) addAccount(provider string) {
	if provider != providerFX && provider != "vkplay" {
		return
	}
	title := "Signing in to VK Play"
	if provider == providerFX {
		title = "Signing in to FX ID"
	}
	g.run(operation{Title: title}, func(s *session) (bool, error) {
		var added account
		var err error
		if provider == providerFX {
			added, err = loginFX(s.p, s.client, s.cfg, "", "")
		} else {
			added, err = loginAccount(s.client, s.cfg, "", vkSignInTimeout)
		}
		if err != nil {
			return false, err
		}
		if name := s.p.line("Account name (leave empty to keep " + added.label() + "):"); name != "" {
			return false, g.store.updateAccount(added.UserID, func(a *account) { a.Name = name })
		}
		return false, nil
	})
}

func (g *guiApp) remove(acc account) {
	g.run(accountOp("Removing "+acc.label(), acc), func(*session) (bool, error) {
		var err error
		if acc.isFX() {
			err = clearRefreshToken(acc.UserID)
		} else {
			err = dropSession(acc.UserID)
		}
		if err != nil {
			return false, err
		}
		return false, g.store.update(func(c *launcherConfig) { c.remove(acc.UserID) })
	})
}

func (g *guiApp) showBranches(acc account) {
	if acc.isFX() {
		g.run(accountOp("Loading branches of "+acc.label(), acc), func(s *session) (bool, error) {
			s.showBranches(acc)
			return false, nil
		})
	}
}

func (g *guiApp) activateKey(acc account) {
	if acc.isFX() {
		g.run(accountOp("Activating a key for "+acc.label(), acc), func(s *session) (bool, error) {
			s.activateKey(acc)
			return false, nil
		})
	}
}

func (g *guiApp) pin(acc account) {
	if err := g.store.update(func(c *launcherConfig) { c.LastUserID = acc.UserID }); err != nil {
		g.notice(err.Error())
		return
	}
	g.emitState(false)
	g.notice(acc.displayName() + " is the main account")
}

func (g *guiApp) rename(id, name string) {
	acc, ok := g.findAccount(id)
	if !ok {
		return
	}
	name = strings.TrimSpace(name)
	if err := g.store.updateAccount(acc.UserID, func(a *account) { a.Name = name }); err != nil {
		g.notice(err.Error())
		return
	}
	acc.Name = name
	g.emitState(false)
	g.notice("Renamed to " + acc.label())
}

func (g *guiApp) watchGame() {
	for {
		g.checkGame()
		time.Sleep(2 * time.Second)
	}
}

func (g *guiApp) checkGame() {
	running := gameRunning()
	if g.gameUp.Swap(running) != running {
		g.emit(map[string]any{"type": "game", "running": running})
		g.updateTray()
	}
}

func (g *guiApp) changeGameFolder() {
	const wait = "Wait until the game operation finishes"
	if g.gameBusy() {
		g.notice(wait)
		return
	}
	dir, err := pickFolder(g.win.hwnd, "Main game folder", g.store.get().Game)
	if err != nil {
		return
	}
	if g.gameBusy() {
		g.notice(wait)
		return
	}
	if state, ok := readBranchState(dir); ok && state.Branch != fxDefaultBranch {
		g.notice(fmt.Sprintf("That folder contains the FX ID %s branch. Choose another folder.", state.Branch))
		return
	}
	g.session.found.game = nil
	if err := g.store.update(func(c *launcherConfig) { c.Game = dir }); err != nil {
		g.notice(err.Error())
		return
	}
	fmt.Println("Main game folder:", dir)
	g.emitState(false)
}

type opOutput struct {
	mu    sync.Mutex
	lines []string
}

func (o *opOutput) note(text string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimRight(line, "\r"); strings.TrimSpace(line) != "" {
			o.lines = append(o.lines, line)
		}
	}
	if len(o.lines) > contextLines {
		o.lines = o.lines[len(o.lines)-contextLines:]
	}
}

func (o *opOutput) take() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	lines := o.lines
	o.lines = nil
	return lines
}

func (g *guiApp) opSession(op operation) *session {
	out := &opOutput{}
	return g.session.with(prompter{
		ask: func(question string) string {
			r := g.prompt("ask", question, false, op.Title, out.take())
			if !r.ok {
				return ""
			}
			return r.value
		},
		confirm: func(question string, def bool) bool {
			r := g.prompt("confirm", question, def, op.Title, out.take())
			return r.ok && r.value == "yes"
		},
		note: out.note,
	})
}

func (g *guiApp) prompt(kind, question string, def bool, op string, context []string) promptReply {
	p := &pendingPrompt{kind: kind, question: strings.TrimSpace(question), def: def, op: op, context: context, reply: make(chan promptReply, 1)}
	g.promptMu.Lock()
	g.nextPrompt++
	id := g.nextPrompt
	g.prompts[id] = p
	g.promptMu.Unlock()
	fmt.Fprintf(os.Stdout, "%s%d\n", promptMarker, id)
	return <-p.reply
}

func (g *guiApp) openPrompt(id int) {
	g.promptMu.Lock()
	p := g.prompts[id]
	g.promptMu.Unlock()
	if p == nil {
		return
	}
	event := map[string]any{
		"type": "prompt", "id": id, "kind": p.kind, "question": p.question, "default": p.def, "op": p.op, "context": p.context,
	}
	if p.kind == "ask" && strings.Contains(strings.ToLower(p.question), "folder") {
		event["folder"] = true
		event["suggest"] = suggestGameFolder(g.store.get().Game)
	}
	g.emit(event)
	g.post(g.showWindow)
}

type folderInfo struct {
	Type    string `json:"type"`
	Prompt  int    `json:"prompt"`
	Path    string `json:"path"`
	Valid   bool   `json:"valid"`
	Free    int64  `json:"free"`
	Drive   string `json:"drive"`
	Install bool   `json:"install"`
	Branch  string `json:"branch"`
	Used    bool   `json:"used"`
}

func describeFolder(prompt int, dir string) folderInfo {
	info := folderInfo{Type: "folderInfo", Prompt: prompt, Path: dir}
	dir = strings.Trim(strings.TrimSpace(dir), `"'`)
	if dir == "" || !filepath.IsAbs(dir) {
		return info
	}
	info.Drive = filepath.VolumeName(dir)
	if _, err := os.Stat(info.Drive + `\`); err != nil {
		return info
	}
	info.Valid = true
	info.Free, _ = diskFree(dir)
	if state, ok := readBranchState(dir); ok {
		if state.Branch == fxDefaultBranch {
			info.Install = true
		} else {
			info.Branch = state.Branch
		}
	}
	info.Install = info.Install || isGameDir(dir)
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		info.Used = true
	}
	return info
}

func suggestGameFolder(saved string) string {
	if saved != "" {
		return saved
	}
	best, bestFree := `C:\Games\Armored Warfare`, int64(-1)
	for _, root := range fixedDrives() {
		if free, err := diskFree(root); err == nil && free > bestFree {
			best, bestFree = filepath.Join(root, "Games", "Armored Warfare"), free
		}
	}
	return best
}

func (g *guiApp) answer(id int, value string, ok bool) {
	g.promptMu.Lock()
	p := g.prompts[id]
	delete(g.prompts, id)
	g.promptMu.Unlock()
	if p != nil {
		p.reply <- promptReply{value: value, ok: ok}
	}
}

func (g *guiApp) captureOutput() error {
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	os.Stdout, os.Stderr = writer, writer
	go func() {
		lines := bufio.NewReader(reader)
		for {
			line, err := lines.ReadString('\n')
			if line != "" {
				g.output(line)
			}
			if err != nil {
				return
			}
		}
	}()
	return nil
}

func (g *guiApp) output(line string) {
	if rest, ok := strings.CutPrefix(line, promptMarker); ok {
		id, _ := strconv.Atoi(strings.TrimSpace(rest))
		g.openPrompt(id)
		return
	}
	line = strings.TrimRight(line, "\r\n") + "\n"
	g.outMu.Lock()
	g.logText = append(g.logText, line...)
	if len(g.logText) > logLimit {
		cut := len(g.logText) - logLimit*3/4
		if i := strings.IndexByte(string(g.logText[cut:]), '\n'); i >= 0 {
			cut += i + 1
		}
		g.logText = append([]byte(nil), g.logText[cut:]...)
	}
	g.outMu.Unlock()
	g.emit(map[string]any{"type": "log", "text": line})
}

func (g *guiApp) watchProgress() {
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	wasActive := false
	for range tick.C {
		s := progress.Default.Snapshot()
		if s.Active || s.Paused || wasActive {
			g.emit(struct {
				Type string `json:"type"`
				progress.Snapshot
			}{"progress", s})
		}
		wasActive = s.Active || s.Paused
	}
}

func systemDarkTheme() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && v == 0
}

func parseHexColor(s string) ([3]uint8, bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return [3]uint8{}, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return [3]uint8{}, false
	}
	return [3]uint8{uint8(v >> 16), uint8(v >> 8), uint8(v)}, true
}

func colorRef(c [3]uint8) uint32 { return uint32(c[0]) | uint32(c[1])<<8 | uint32(c[2])<<16 }
