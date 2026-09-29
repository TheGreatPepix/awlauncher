package launcher

import (
	"errors"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/fxid"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/vkplay"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
)

func (s *Session) AddAccount(provider string) error {
	var added config.Account
	var err error
	switch provider {
	case config.ProviderFX:
		added, err = s.loginFX("", "")
	case "vkplay":
		added, err = s.loginVK("")
	default:
		return errors.New("unknown service " + provider)
	}
	if err != nil {
		return err
	}
	name, ok := s.ui.Ask(Prompt{Kind: PromptName, Question: "Account name (leave empty to keep " + added.Label() + ")"})
	if name = strings.TrimSpace(name); ok && name != "" {
		return s.Rename(added, name)
	}
	return nil
}

func (s *Session) RemoveAccount(acc config.Account) error {
	var err error
	if acc.IsFX() {
		err = fxid.SignOut(s.client, acc.UserID, platform.Language())
	} else {
		err = vkplay.SignOut(s.client, acc.UserID)
	}
	if err != nil {
		return err
	}
	return s.cfg.Update(func(c *config.Config) { c.Remove(acc.UserID) })
}

func (s *Session) Relogin(acc config.Account) (config.Account, error) {
	s.ui.Sayf("Session for %s has expired, sign in again.", acc.Label())
	var relogged config.Account
	var err error
	if acc.IsFX() {
		relogged, err = s.loginFX(acc.Email, acc.Name)
	} else {
		relogged, err = s.loginVK(acc.Name)
	}
	if err != nil {
		return config.Account{}, err
	}
	if relogged.UserID != acc.UserID {
		s.ui.Sayf("Signed in to a different account (%s); it was added to the list.", relogged.Label())
	}
	return relogged, nil
}
