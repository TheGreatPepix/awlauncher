package gui

import (
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

const waitForGame = "Wait until the game operation finishes"

type host interface {
	post(f func())
	eval(script string)
	showWindow()
	hideWindow()
	windowShown() bool
	exit()
	setTitleBar(dark bool, caption, text [3]uint8)
	pickFolder(initial string) (string, error)
	copyText(text string) error
	open(target string)
	setTrayPlay(text string, enabled bool)
	relabelTray()
	openSignIn(url string, report func(code, state string), closed func()) error
	showSignIn()
	closeSignIn()
}

type App struct {
	host    host
	store   *config.Store
	session *launcher.Session
	ops     launcher.Ops
	logs    *logBuffer
	gameUp  atomic.Bool

	pageReady atomic.Bool
	updated   bool
	checked   bool
	signIn    *signIn

	promptMu   sync.Mutex
	prompts    map[int]*pendingPrompt
	nextPrompt int
}

func newApp(store *config.Store, logs *logBuffer, updated bool) *App {
	return &App{store: store, session: launcher.NewSession(store), logs: logs, updated: updated, prompts: map[int]*pendingPrompt{}}
}

func (g *App) emit(event any) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	g.host.post(func() {
		if g.pageReady.Load() {
			g.host.eval("window.aw && aw.recv(" + string(data) + ")")
		}
	})
}

func (g *App) emitLog(line string) { g.emit(map[string]any{"type": "log", "text": line}) }

func (g *App) notice(message string) {
	g.emit(map[string]any{"type": "notice", "message": message})
}

func (g *App) run(op launcher.Operation, done string, action func(s *launcher.Session) (bool, error)) {
	if reason := g.ops.Begin(&op); reason != "" {
		g.notice(reason)
		return
	}
	g.emitState(false)
	s := g.session.WithUI(g.opUI(op.Title))
	go func() {
		launched, err := action(s)
		g.host.post(func() {
			others := g.ops.Finish(op.ID)
			result := map[string]any{"type": "done", "id": op.ID, "title": op.Title}
			switch {
			case errors.Is(err, launcher.ErrCancelled):
				result["status"] = "cancelled"
			case err != nil:
				message := strings.TrimSpace(err.Error())
				result["status"] = "error"
				result["message"] = message
				log.Print("Error: ", message)
				g.host.showWindow()
			case launched:
				result["status"] = "launched"
				if others == 0 && !g.store.Get().KeepOpen {
					result["hidden"] = true
					g.host.hideWindow()
				}
			default:
				result["status"] = "ok"
				if done != "" {
					result["message"] = done
				}
			}
			g.emitState(false)
			g.emit(result)
		})
	}()
}

func (g *App) play(acc config.Account) {
	op := launcher.AccountOp("Starting "+acc.Label(), acc)
	op.Game = true
	g.run(op, "", func(s *launcher.Session) (bool, error) {
		return true, s.Play(acc)
	})
}

func (g *App) requestExit() {
	if !g.pageReady.Load() || len(g.ops.List()) == 0 {
		g.host.exit()
		return
	}
	g.host.showWindow()
	g.emit(map[string]any{"type": "confirmExit"})
}

func (g *App) playLast() {
	if acc, ok := g.store.Find(g.store.Get().LastUserID); ok {
		g.play(acc)
	}
}

func (g *App) addAccount(provider string) {
	title := "Signing in to VK Play"
	switch provider {
	case config.ProviderFX:
		title = "Signing in to FX ID"
	case "vkplay":
	default:
		return
	}
	g.run(launcher.Operation{Title: title}, "Account added", func(s *launcher.Session) (bool, error) {
		return false, s.AddAccount(provider)
	})
}

func (g *App) removeAccount(acc config.Account) {
	g.run(launcher.AccountOp("Removing "+acc.Label(), acc), "Account removed", func(s *launcher.Session) (bool, error) {
		return false, s.RemoveAccount(acc)
	})
}

func (g *App) chooseBranch(acc config.Account) {
	if acc.IsFX() {
		g.run(launcher.AccountOp("Loading branches of "+acc.Label(), acc), "", func(s *launcher.Session) (bool, error) {
			return false, s.ChooseBranch(acc)
		})
	}
}

func (g *App) activateKey(acc config.Account) {
	if acc.IsFX() {
		g.run(launcher.AccountOp("Activating a key for "+acc.Label(), acc), "", func(s *launcher.Session) (bool, error) {
			return false, s.ActivateKey(acc)
		})
	}
}

func (g *App) pin(acc config.Account) {
	if err := g.session.Pin(acc); err != nil {
		g.notice(err.Error())
		return
	}
	g.emitState(false)
	g.notice(acc.DisplayName() + " is the main account")
}

func (g *App) rename(acc config.Account, name string) {
	name = strings.TrimSpace(name)
	if err := g.session.Rename(acc, name); err != nil {
		g.notice(err.Error())
		return
	}
	acc.Name = name
	g.emitState(false)
	g.notice("Renamed to " + acc.Label())
}

func (g *App) setGameLanguage(acc config.Account, code string) {
	if err := g.session.SetGameLanguage(acc, code); err != nil {
		g.notice(err.Error())
		return
	}
	g.emitState(false)
}

func (g *App) closeGame() {
	if err := g.session.CloseGame(); err != nil {
		g.notice(err.Error())
	} else {
		g.notice("The game is closed")
	}
	g.checkGame()
}

func (g *App) watchGame() {
	for {
		g.checkGame()
		time.Sleep(2 * time.Second)
	}
}

func (g *App) checkGame() {
	running := platform.GameRunning()
	if g.gameUp.Swap(running) != running {
		g.emit(map[string]any{"type": "game", "running": running})
		g.updateTray()
	}
}

func (g *App) watchProgress() {
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	wasActive := false
	for range tick.C {
		if !g.host.windowShown() {
			continue
		}
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

func (g *App) findAccount(id string) (config.Account, bool) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return config.Account{}, false
	}
	return g.store.Find(n)
}
