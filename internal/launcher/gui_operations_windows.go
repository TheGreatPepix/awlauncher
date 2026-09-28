package launcher

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/fxid"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
)

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

func accountOp(title string, acc config.Account) operation {
	return operation{Title: title, Account: strconv.FormatInt(acc.UserID, 10)}
}

func (g *guiApp) play(acc config.Account) {
	op := accountOp("Starting "+acc.Label(), acc)
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
		return true, g.store.Update(func(c *config.Config) { c.LastUserID = acc.UserID })
	})
}

func (g *guiApp) addAccount(provider string) {
	if provider != config.ProviderFX && provider != "vkplay" {
		return
	}
	title := "Signing in to VK Play"
	if provider == config.ProviderFX {
		title = "Signing in to FX ID"
	}
	g.run(operation{Title: title}, func(s *session) (bool, error) {
		var added config.Account
		var err error
		if provider == config.ProviderFX {
			added, err = loginFX(s.p, s.client, s.cfg, "", "")
		} else {
			added, err = loginAccount(s.client, s.cfg, "", vkSignInTimeout)
		}
		if err != nil {
			return false, err
		}
		if name := s.p.line("Account name (leave empty to keep " + added.Label() + "):"); name != "" {
			return false, g.store.UpdateAccount(added.UserID, func(a *config.Account) { a.Name = name })
		}
		return false, nil
	})
}

func (g *guiApp) remove(acc config.Account) {
	g.run(accountOp("Removing "+acc.Label(), acc), func(*session) (bool, error) {
		var err error
		if acc.IsFX() {
			err = config.ClearRefreshToken(acc.UserID)
		} else {
			err = dropSession(acc.UserID)
		}
		if err != nil {
			return false, err
		}
		return false, g.store.Update(func(c *config.Config) { c.Remove(acc.UserID) })
	})
}

func (g *guiApp) showBranches(acc config.Account) {
	if acc.IsFX() {
		g.run(accountOp("Loading branches of "+acc.Label(), acc), func(s *session) (bool, error) {
			s.showBranches(acc)
			return false, nil
		})
	}
}

func (g *guiApp) activateKey(acc config.Account) {
	if acc.IsFX() {
		g.run(accountOp("Activating a key for "+acc.Label(), acc), func(s *session) (bool, error) {
			s.activateKey(acc)
			return false, nil
		})
	}
}

func (g *guiApp) pin(acc config.Account) {
	if err := g.store.Update(func(c *config.Config) { c.LastUserID = acc.UserID }); err != nil {
		g.notice(err.Error())
		return
	}
	g.emitState(false)
	g.notice(acc.DisplayName() + " is the main account")
}

func (g *guiApp) rename(id, name string) {
	acc, ok := g.findAccount(id)
	if !ok {
		return
	}
	name = strings.TrimSpace(name)
	if err := g.store.UpdateAccount(acc.UserID, func(a *config.Account) { a.Name = name }); err != nil {
		g.notice(err.Error())
		return
	}
	acc.Name = name
	g.emitState(false)
	g.notice("Renamed to " + acc.Label())
}

func (g *guiApp) watchGame() {
	for {
		g.checkGame()
		time.Sleep(2 * time.Second)
	}
}

func (g *guiApp) checkGame() {
	running := platform.GameRunning()
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
	r := g.prompt("ask", "VK Play game folder: ", false, "VK Play game folder", nil)
	if !r.ok {
		return
	}
	dir := strings.Trim(strings.TrimSpace(r.value), `"'`)
	if !describeFolder(0, dir).Valid {
		g.notice("Enter a full path on an existing drive, like D:\\Games\\Armored Warfare.")
		return
	}
	if g.gameBusy() {
		g.notice(wait)
		return
	}
	if _, ok := gamefiles.ReadBranchState(dir); ok || strings.EqualFold(dir, g.store.Get().FXGame) {
		g.notice("That folder contains an FX ID client. Choose another folder.")
		return
	}
	g.session.found.game = nil
	if err := g.store.Update(func(c *config.Config) { c.Game = dir }); err != nil {
		g.notice(err.Error())
		return
	}
	fmt.Println("VK Play game folder:", dir)
	g.emitState(false)
}

func (g *guiApp) changeFXFolder() {
	if g.gameBusy() {
		g.notice("Wait until the game operation finishes")
		return
	}
	r := g.prompt("ask", "FX ID game folder: ", false, "FX ID game folder", nil)
	if !r.ok {
		return
	}
	dir := strings.Trim(strings.TrimSpace(r.value), `"'`)
	if !describeFolder(0, dir).Valid {
		g.notice("Enter a full path on an existing drive, like D:\\Games\\Armored Warfare.")
		return
	}
	dir = filepath.Clean(dir)
	if strings.EqualFold(dir, g.store.Get().Game) || gamefiles.IsVKInstall(dir) {
		g.notice("Choose a folder different from the VK Play client.")
		return
	}
	for _, branchDir := range g.store.Get().BranchGames {
		if strings.EqualFold(dir, branchDir) {
			g.notice("That folder is assigned to a closed branch.")
			return
		}
	}
	if state, ok := gamefiles.ReadBranchState(dir); ok && state.Branch != gamefiles.DefaultBranch {
		g.notice("That folder contains a closed FX ID branch.")
		return
	}
	if g.gameBusy() {
		g.notice("Wait until the game operation finishes")
		return
	}
	if err := g.store.Update(func(c *config.Config) { c.FXGame = dir }); err != nil {
		g.notice(err.Error())
		return
	}
	g.emitState(false)
	g.emit(describeConfiguredGame(g.store.Get()))
}

func (g *guiApp) changeBranchFolder(branch string) {
	if !fxid.ValidBranchName(branch) || strings.EqualFold(branch, gamefiles.DefaultBranch) {
		return
	}
	if g.gameBusy() {
		g.notice("Wait until the game operation finishes")
		return
	}
	cfg := g.store.Get()
	current := cfg.BranchDir(branch)
	r := g.prompt("ask", "Game folder for FX ID "+branch+" ["+current+"]: ", false, "FX ID "+branch, nil)
	if !r.ok {
		return
	}
	dir := strings.Trim(strings.TrimSpace(r.value), `"'`)
	if dir == "" {
		dir = current
	}
	if !describeFolder(0, dir).Valid {
		g.notice("Enter a full path on an existing drive, like D:\\Games\\Armored Warfare.")
		return
	}
	dir = filepath.Clean(dir)
	if strings.EqualFold(dir, cfg.Game) {
		g.notice("Choose a folder different from the VK Play folder.")
		return
	}
	if strings.EqualFold(dir, cfg.FXGame) {
		g.notice("Choose a folder different from the FX ID main client.")
		return
	}
	for other, saved := range cfg.BranchGames {
		if !strings.EqualFold(other, branch) && strings.EqualFold(dir, saved) {
			g.notice("That folder is assigned to another branch.")
			return
		}
	}
	if state, ok := gamefiles.ReadBranchState(dir); ok && !strings.EqualFold(state.Branch, branch) {
		g.notice("That folder contains a different FX ID branch.")
		return
	}
	if gamefiles.IsVKInstall(dir) {
		g.notice("That folder contains the VK Play client.")
		return
	}
	if g.gameBusy() {
		g.notice("Wait until the game operation finishes")
		return
	}
	if err := g.store.Update(func(c *config.Config) {
		if c.BranchGames == nil {
			c.BranchGames = map[string]string{}
		}
		c.BranchGames[strings.ToLower(branch)] = dir
	}); err != nil {
		g.notice(err.Error())
		return
	}
	g.emitState(false)
	g.emit(describeConfiguredGame(g.store.Get()))
}
