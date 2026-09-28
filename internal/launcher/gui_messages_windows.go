package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/startup"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/ui"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/update"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

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
		if g.updated {
			g.updated = false
			fmt.Println("AWLauncher is updated to", Version+".")
			g.notice("AWLauncher is updated to " + Version)
		}
		if !g.checked && update.ReleaseBuild(Version) {
			g.checked = true
			go func() {
				info := update.Check(authClient(), Version)
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
		if err := startup.Set(c.Value); err != nil {
			g.notice("Cannot change the start with Windows: " + err.Error())
		}
		g.emitState(false)
	case "checkUpdate":
		go func() { g.emit(update.Check(authClient(), Version)) }()
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
		if ui.SetLanguage(c.Lang) && g.tray != nil {
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
	case "copyLogs":
		err := copyTextToClipboard(g.win.hwnd, c.Value)
		if err != nil {
			fmt.Println("Cannot copy logs:", err)
		}
		g.emit(map[string]any{"type": "clipboard", "ok": err == nil})
	case "gameInfo":
		go func() { g.emit(describeConfiguredGame(g.store.Get())) }()
	case "availableClients":
		go func() { g.emit(availableGameClients(g.session)) }()
	case "downloadClient":
		acc, ok := g.findAccount(c.Account)
		if !ok {
			return
		}
		kind, branch := c.Value, c.Branch
		op := accountOp("Downloading game", acc)
		op.Game = true
		g.run(op, func(s *session) (bool, error) {
			err := s.downloadClient(acc, kind, branch)
			if errors.Is(err, errNeedLogin) {
				relogged, loginErr := s.relogin(acc)
				if loginErr != nil {
					return false, loginErr
				}
				err = s.downloadClient(relogged, kind, branch)
			}
			return false, err
		})
	case "verify", "removeBranch", "removeMainClient", "updateClient", "openClientFolder", "clearDownloads":
		g.clientCommand(c.Cmd, c.Value, c.Branch)
	case "gameFolder":
		go g.changeGameFolder()
	case "fxFolder":
		go g.changeFXFolder()
	case "allowMods":
		if g.gameBusy() {
			g.notice("Wait until the game operation finishes")
			return
		}
		if err := g.store.Update(func(cfg *config.Config) { cfg.AllowMods = c.Value == "on" }); err != nil {
			g.notice(err.Error())
			return
		}
		g.emitState(false)
	case "branchFolder":
		go g.changeBranchFolder(c.Branch)
	case "openDataFolder":
		if dir, err := platform.DataDir(); err == nil {
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
			if err := platform.CloseGame(10 * time.Second); err != nil {
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
	Type          string      `json:"type"`
	Accounts      []uiAccount `json:"accounts"`
	Game          string      `json:"game"`
	SuggestedGame string      `json:"suggestedGame"`
	FXGame        string      `json:"fxGame"`
	AllowMods     bool        `json:"allowMods"`
	Data          string      `json:"data"`
	Version       string      `json:"version"`
	Autostart     string      `json:"autostart"`
	SystemLang    string      `json:"systemLang"`
	Ops           []operation `json:"ops"`
	Running       bool        `json:"running"`
	Log           *string     `json:"log,omitempty"`
	Prefs         *uiPrefs    `json:"prefs,omitempty"`
}

func accountViews(cfg config.Config) []uiAccount {
	views := make([]uiAccount, 0, len(cfg.Accounts))
	for _, a := range cfg.Accounts {
		login, name := a.Login(), a.DisplayName()
		provider := "vkplay"
		if a.IsFX() {
			provider = "fxid"
		}
		views = append(views, uiAccount{
			ID: strconv.FormatInt(a.UserID, 10), Name: name, Login: login, Service: a.Service(),
			Provider: provider, Branch: a.Branch, Last: a.UserID == cfg.LastUserID,
		})
	}
	return views
}

func (g *guiApp) emitState(withLog bool) {
	cfg := g.store.Get()
	g.opsMu.Lock()
	ops := append([]operation{}, g.ops...)
	g.opsMu.Unlock()
	s := uiState{Type: "state", Accounts: accountViews(cfg), Game: cfg.Game, FXGame: cfg.FXGame, AllowMods: cfg.AllowMods, SuggestedGame: suggestGameFolder(cfg.Game), Ops: ops, Running: g.gameUp.Load(), Version: Version, Autostart: startup.Mode(), SystemLang: ui.Language()}
	if dir, err := platform.DataDir(); err == nil {
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
	acc, ok := g.store.Find(g.store.Get().LastUserID)
	if !ok {
		g.tray.setPlay("", false)
		return
	}
	g.tray.setPlay(ui.Translate("Play")+" "+acc.DisplayName()+" · "+acc.Service(), !g.gameBusy() && !g.gameUp.Load())
}

func (g *guiApp) clientCommand(cmd, kind, branch string) {
	c, err := g.session.findClient(kind, branch)
	if err != nil {
		g.notice(err.Error())
		return
	}
	switch cmd {
	case "openClientFolder":
		openInShell(c.Dir)
		return
	case "clearDownloads":
		g.run(operation{Title: "Deleting downloaded patches", Game: true}, func(s *session) (bool, error) {
			return false, s.clearDownloads(c.Dir)
		})
		return
	case "updateClient":
		g.run(operation{Title: "Updating " + c.Name(), Game: true}, func(s *session) (bool, error) {
			msg, err := s.updateClient(c)
			if err == nil && msg != "" {
				s.p.Notify(msg)
			}
			return false, err
		})
		return
	}
	if cmd == "verify" {
		g.run(operation{Title: "Checking " + c.Name(), Game: true}, func(s *session) (bool, error) {
			return false, s.verifyClient(c)
		})
		return
	}
	if cmd == "removeMainClient" {
		g.run(operation{Title: "Removing " + c.Name(), Game: true}, func(s *session) (bool, error) {
			return false, s.removeMainClient(c)
		})
		return
	}
	g.run(operation{Title: "Removing " + c.Name(), Game: true}, func(s *session) (bool, error) {
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
		exe, err := update.Install(&http.Client{Timeout: 10 * time.Minute}, Version)
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
			if err := update.StartUpdated(exe, func(pid int) { procAllowSetForegroundWindo.Call(uintptr(pid)) }); err != nil {
				fmt.Println("Cannot restart AWLauncher:", err)
				return
			}
			g.exit()
		})
		return false, nil
	})
}

func (g *guiApp) playLast() {
	if acc, ok := g.store.Find(g.store.Get().LastUserID); ok {
		g.play(acc)
	}
}

func (g *guiApp) findAccount(id string) (config.Account, bool) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return config.Account{}, false
	}
	return g.store.Find(n)
}
