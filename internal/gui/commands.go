package gui

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/gui/startup"
	"github.com/TheGreatPepix/awlauncher/internal/gui/ui"
	"github.com/TheGreatPepix/awlauncher/internal/gui/update"
	"github.com/TheGreatPepix/awlauncher/internal/launcher"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/fxid"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

type uiCommand struct {
	Cmd      string      `json:"cmd"`
	Account  string      `json:"account"`
	Provider string      `json:"provider"`
	Prompt   int         `json:"prompt"`
	Value    string      `json:"value"`
	OK       bool        `json:"ok"`
	Dark     bool        `json:"dark"`
	Caption  string      `json:"caption"`
	Text     string      `json:"text"`
	Theme    string      `json:"theme"`
	Branch   string      `json:"branch"`
	Hue      int         `json:"hue"`
	Style    string      `json:"style"`
	Palettes []uiPalette `json:"palettes"`
	Lang     string      `json:"lang"`
}

func (g *App) onMessage(message string) {
	var c uiCommand
	if err := json.Unmarshal([]byte(message), &c); err != nil {
		return
	}
	switch c.Cmd {
	case "ready":
		g.pageReady.Store(true)
		g.emitState(true)
		g.resendPrompts()
		if g.updated {
			g.updated = false
			log.Print("AWLauncher is updated to ", launcher.Version, ".")
			g.notice("AWLauncher is updated to " + launcher.Version)
		}
		if !g.checked && update.ReleaseBuild(launcher.Version) {
			g.checked = true
			go func() {
				info := update.Check(launcher.HTTPClient(), launcher.Version)
				info.Quiet = true
				g.emit(info)
			}()
		}
	case "pause":
		if progress.Default.Pause() {
			log.Print("Paused. Press Resume to go on from where it stopped.")
		}
	case "resume":
		if progress.Default.Resume() {
			log.Print("Resumed.")
		}
	case "autostart":
		if err := startup.Set(c.Value); err != nil {
			g.notice("Cannot change the start with Windows: " + err.Error())
		}
		g.emitState(false)
	case "checkUpdate":
		go func() { g.emit(update.Check(launcher.HTTPClient(), launcher.Version)) }()
	case "applyUpdate":
		g.updateSelf()
	case "theme":
		caption, ok1 := parseHexColor(c.Caption)
		text, ok2 := parseHexColor(c.Text)
		if ok1 && ok2 {
			g.host.setTitleBar(c.Dark, caption, text)
		}
	case "prefs":
		if err := savePrefs(uiPrefs{Theme: c.Theme, Hue: c.Hue, Style: c.Style, Palettes: c.Palettes, Lang: c.Lang}); err != nil {
			log.Print("Cannot save the appearance settings: ", err)
		}
		if ui.SetLanguage(c.Lang) {
			g.host.relabelTray()
			g.updateTray()
		}
	case "answer":
		g.answer(c.Prompt, c.Value, c.OK)
	case "browse":
		id, initial := c.Prompt, c.Value
		g.host.post(func() {
			if dir, err := g.host.pickFolder(initial); err == nil {
				g.emit(map[string]any{"type": "browsed", "prompt": id, "path": dir})
			}
		})
	case "folderInfo":
		id, dir := c.Prompt, c.Value
		go func() {
			g.emit(struct {
				Type   string `json:"type"`
				Prompt int    `json:"prompt"`
				launcher.FolderInfo
			}{"folderInfo", id, launcher.DescribeFolder(dir)})
		}()
	case "copyLogs":
		err := g.host.copyText(c.Value)
		if err != nil {
			log.Print("Cannot copy logs: ", err)
		}
		g.emit(map[string]any{"type": "clipboard", "ok": err == nil})
	case "gameInfo":
		go g.emitGameInfo()
	case "availableClients":
		go func() {
			g.emit(struct {
				Type string `json:"type"`
				launcher.AvailableClients
			}{"availableClients", g.session.AvailableClients()})
		}()
	case "downloadClient":
		if acc, ok := g.findAccount(c.Account); ok {
			kind, branch := c.Value, c.Branch
			op := launcher.AccountOp("Downloading game", acc)
			op.Game = true
			g.run(op, "", func(s *launcher.Session) (bool, error) {
				return false, s.DownloadClient(acc, kind, branch)
			})
		}
	case "verify", "removeBranch", "removeMainClient", "updateClient", "openClientFolder", "clearDownloads":
		g.clientCommand(c.Cmd, c.Value, c.Branch)
	case "gameFolder":
		go g.changeFolder(gamefiles.KindVK, "")
	case "fxFolder":
		go g.changeFolder(gamefiles.KindFX, "")
	case "branchFolder":
		go g.changeFolder(gamefiles.KindBranch, c.Branch)
	case "allowMods":
		if g.ops.GameBusy() {
			g.notice(waitForGame)
			return
		}
		if err := g.session.SetAllowMods(c.Value == "on"); err != nil {
			g.notice(err.Error())
			return
		}
		g.emitState(false)
	case "patchBackups":
		if g.ops.GameBusy() {
			g.notice(waitForGame)
			return
		}
		if err := g.session.SetPatchBackups(c.Value == "on"); err != nil {
			g.notice(err.Error())
			return
		}
		g.emitState(false)
	case "exit":
		g.host.post(g.host.exit)
	case "openDataFolder":
		if dir, err := platform.DataDir(); err == nil {
			g.host.open(dir)
		}
	case "openLink":
		if strings.HasPrefix(c.Value, "https://") {
			g.host.open(c.Value)
		}
	case "add":
		g.addAccount(c.Provider)
	case "signinOpen", "signinFresh", "signinHere", "signinCancel":
		cmd := c.Cmd
		g.host.post(func() { g.signInCommand(cmd) })
	case "closeGame":
		go g.closeGame()
	case "play", "remove", "branches", "key", "pin", "rename", "language":
		acc, ok := g.findAccount(c.Account)
		if !ok {
			return
		}
		switch c.Cmd {
		case "play":
			g.play(acc)
		case "remove":
			g.removeAccount(acc)
		case "branches":
			g.chooseBranch(acc)
		case "key":
			g.activateKey(acc)
		case "pin":
			g.pin(acc)
		case "rename":
			g.rename(acc, c.Value)
		case "language":
			g.setGameLanguage(acc, c.Value)
		}
	}
}

func (g *App) emitGameInfo() {
	g.emit(struct {
		Type string `json:"type"`
		launcher.GameInfo
	}{"gameInfo", g.session.GameInfo()})
}

func (g *App) clientCommand(cmd, kind, branch string) {
	c, err := g.session.FindClient(kind, branch)
	if err != nil {
		g.notice(err.Error())
		return
	}
	op := launcher.Operation{Game: true}
	var done string
	var action func(s *launcher.Session) error
	switch cmd {
	case "openClientFolder":
		g.host.open(c.Dir)
		return
	case "clearDownloads":
		op.Title = "Deleting downloaded patches"
		action = func(s *launcher.Session) error { return s.ClearDownloads(c.Dir) }
	case "updateClient":
		op.Title = "Updating " + c.Name()
		action = func(s *launcher.Session) error { return s.UpdateClient(c) }
	case "verify":
		op.Title = "Checking " + c.Name()
		action = func(s *launcher.Session) error { return s.VerifyClient(c) }
	case "removeMainClient":
		op.Title, done = "Removing "+c.Name(), "Client removed"
		action = func(s *launcher.Session) error { return s.RemoveMainClient(c) }
	case "removeBranch":
		op.Title, done = "Removing "+c.Name(), "Client removed"
		action = func(s *launcher.Session) error { return s.RemoveBranch(c) }
	default:
		return
	}
	g.run(op, done, func(s *launcher.Session) (bool, error) { return false, action(s) })
}

func (g *App) changeFolder(kind, branch string) {
	if kind == gamefiles.KindBranch && (!fxid.ValidBranchName(branch) || strings.EqualFold(branch, gamefiles.DefaultBranch)) {
		return
	}
	if g.ops.GameBusy() {
		g.notice(waitForGame)
		return
	}
	prompt := g.session.FolderPrompt(kind, branch)
	u := g.opUI(prompt.Question)
	dir, ok := u.Ask(prompt)
	if !ok {
		return
	}
	if g.ops.GameBusy() {
		g.notice(waitForGame)
		return
	}
	s := g.session.WithUI(u)
	var err error
	switch kind {
	case gamefiles.KindVK:
		err = s.SetVKFolder(dir)
	case gamefiles.KindFX:
		err = s.SetFXFolder(dir)
	default:
		err = s.SetBranchFolder(branch, dir)
	}
	if err != nil {
		g.notice(err.Error())
		return
	}
	g.emitState(false)
	g.emitGameInfo()
}
