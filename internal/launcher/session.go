package launcher

import (
	"errors"
	"log"
	"net/http"
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

type foundGame struct{ game *gamefiles.Install }

func NewSession(cfg *config.Store) *Session {
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
