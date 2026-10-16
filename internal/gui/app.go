package gui

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"runtime"
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
	send(event []byte)
	showWindow()
	hideWindow()
	windowShown() bool
	exit()
	open(target string)
	setTrayPlay(text string, enabled bool)
}

type App struct {
	host    host
	store   *config.Store
	session *launcher.Session
	ops     launcher.Ops
	logs    *logBuffer
	gameUp  atomic.Bool
	discard atomic.Bool // a cancelled download also removes its client

	pageReady atomic.Bool
	appExe    string // the window's executable
	updated   bool
	checked   bool
	signIn    *signIn

	promptMu   sync.Mutex
	prompts    map[int]*pendingPrompt
	nextPrompt int
}

// trayHides says whether the window can hide to the tray after the game
// starts. On Linux there is no tray to bring it back from, and in Steam's
// Game Mode the game takes the screen anyway.
const trayHides = runtime.GOOS == "windows"

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
			g.host.send(data)
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
	if op.Game {
		g.discard.Store(false)
		progress.Default.Arm()
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
				var c cancelled
				if errors.As(err, &c) {
					result["message"] = c.message
				}
			case err != nil:
				message := strings.TrimSpace(err.Error())
				result["status"] = "error"
				result["message"] = message
				log.Print("Error: ", message)
				g.host.showWindow()
			case launched:
				result["status"] = "launched"
				if others == 0 && !g.store.Get().KeepOpen && trayHides {
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

// cancelled ends a download the person cancelled, with a message for them.
type cancelled struct{ message string }

func (c cancelled) Error() string { return c.message }

func (c cancelled) Unwrap() error { return launcher.ErrCancelled }

// cancelDownload stops the running download. A new install is always removed;
// an update removes its client only when the person asks for it.
func (g *App) cancelDownload(remove bool) {
	op, ok := g.ops.Cancellable()
	if !ok {
		return
	}
	g.discard.Store(remove || op.Cancel == launcher.CancelInstall)
	if progress.Default.Cancel() {
		log.Print("Cancelling ", op.Title, "...")
	}
}

// afterCancel runs once a cancellable download returns. All its workers are
// done by then, so nothing writes into the client while it is removed.
func (g *App) afterCancel(err error, discard func() error) error {
	if !progress.Default.Cancelled() {
		return err
	}
	if !g.discard.Load() {
		log.Print("The download is stopped. It goes on from the same place next time.")
		return cancelled{"Download stopped"}
	}
	if err := discard(); err != nil {
		return fmt.Errorf("remove the cancelled client: %w", err)
	}
	return cancelled{"Download cancelled, the client is removed"}
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
	case config.ProviderSteam:
		title = "Signing in to Steam"
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
