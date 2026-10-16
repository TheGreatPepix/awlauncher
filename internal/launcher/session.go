package launcher

import (
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
)

type Session struct {
	ui     UI
	cfg    *config.Store
	client *http.Client
	found  *foundGame
}

// foundGame caches the VK Play install; operations and folder changes run on
// different goroutines, so access goes through a lock.
type foundGame struct {
	mu   sync.Mutex
	game *gamefiles.Install
}

func (f *foundGame) get() *gamefiles.Install {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.game
}

func (f *foundGame) set(g *gamefiles.Install) {
	f.mu.Lock()
	f.game = g
	f.mu.Unlock()
}

func NewSession(cfg *config.Store) *Session {
	if cfg != nil {
		platform.SetProton(cfg.Get().Proton)
	}
	return &Session{cfg: cfg, client: authClient(), found: &foundGame{}}
}

func (s *Session) WithUI(ui UI) *Session {
	c := *s
	c.ui = ui
	return &c
}

func authClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

func HTTPClient() *http.Client { return authClient() }

func (s *Session) withLogin(acc config.Account, f func(config.Account) error) error {
	err := f(acc)
	if errors.Is(err, ErrNeedLogin) {
		relogged, loginErr := s.Relogin(acc)
		if loginErr != nil {
			return loginErr
		}
		err = f(relogged)
	}
	return err
}

func (s *Session) Pin(acc config.Account) error {
	return s.cfg.Update(func(c *config.Config) { c.LastUserID = acc.UserID })
}

func (s *Session) Rename(acc config.Account, name string) error {
	return s.cfg.UpdateAccount(acc.UserID, func(a *config.Account) { a.Name = name })
}

func (s *Session) SetPatchBackups(on bool) error {
	return s.cfg.Update(func(c *config.Config) { c.NoBackups = !on })
}

func (s *Session) SetHideOnLaunch(on bool) error {
	return s.cfg.Update(func(c *config.Config) { c.KeepOpen = !on })
}

func (s *Session) SetProton(path string) error {
	if err := s.cfg.Update(func(c *config.Config) { c.Proton = path }); err != nil {
		return err
	}
	platform.SetProton(path)
	return nil
}

func (s *Session) SetAllowMods(on bool) error {
	return s.cfg.Update(func(c *config.Config) { c.AllowMods = on })
}

func (s *Session) CloseGame() error {
	log.Print("Closing the game...")
	if err := platform.CloseGame(10 * time.Second); err != nil {
		log.Print("Error: ", err)
		return err
	}
	log.Print("The game is closed.")
	return nil
}
