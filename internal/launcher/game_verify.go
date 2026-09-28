package launcher

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/catalog"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
)

func (s *session) findClient(kind, branch string) (gamefiles.Client, error) {
	root := s.cfg.Get().Game
	for _, c := range installedConfiguredClients(s.cfg.Get()) {
		if c.Kind == kind && (kind != gamefiles.KindBranch || strings.EqualFold(c.Branch, branch)) {
			return c, nil
		}
	}
	return gamefiles.Client{}, fmt.Errorf("that client is not installed in %s", root)
}
func (s *session) chooseClient(question string) (gamefiles.Client, error) {
	root := s.cfg.Get().Game
	clients := installedConfiguredClients(s.cfg.Get())
	switch len(clients) {
	case 0:
		return gamefiles.Client{}, fmt.Errorf("the game is not installed in %s", root)
	case 1:
		return clients[0], nil
	}
	for i, c := range clients {
		s.p.Sayf("  %d  %s, %s\n", i+1, c.Name(), c.Version)
	}
	n, err := strconv.Atoi(firstWord(s.p.line(question)))
	if err != nil || n < 1 || n > len(clients) {
		return gamefiles.Client{}, errQuit
	}
	return clients[n-1], nil
}
func (s *session) chooseMainClient() (gamefiles.Client, error) {
	var clients []gamefiles.Client
	for _, c := range installedConfiguredClients(s.cfg.Get()) {
		if c.Kind != gamefiles.KindBranch {
			clients = append(clients, c)
		}
	}
	if len(clients) == 0 {
		return gamefiles.Client{}, errors.New("no main client is installed")
	}
	if len(clients) == 1 {
		return clients[0], nil
	}
	for i, c := range clients {
		s.p.Sayf("  %d  %s, %s: %s\n", i+1, c.Name(), c.Version, c.Dir)
	}
	n, err := strconv.Atoi(firstWord(s.p.line("Client number: ")))
	if err != nil || n < 1 || n > len(clients) {
		return gamefiles.Client{}, errQuit
	}
	return clients[n-1], nil
}
func (s *session) verifyClient(c gamefiles.Client) error {
	if err := gamefiles.EnsureGameClosed(); err != nil {
		return err
	}
	if c.Kind == gamefiles.KindVK {
		return s.verifyVK(c.Dir)
	}
	acc, ok := s.fxAccountFor(c.Branch)
	if !ok {
		return errors.New("add an FX ID account first: the file list comes from FX ID")
	}
	if err := gamefiles.MarkBranchDirty(c.Dir); err != nil {
		return err
	}
	root := s.cfg.Get().FXGame
	state, err := syncFXClient(s.client, &s.p, acc, c.Branch, c.Dir, root, false, false)
	if errors.Is(err, errNeedLogin) {
		if acc, err = s.relogin(acc); err != nil {
			return err
		}
		state, err = syncFXClient(s.client, &s.p, acc, c.Branch, c.Dir, root, false, false)
	}
	if err != nil {
		return err
	}
	s.p.Notify(fmt.Sprintf("%s %s: all files are checked", c.Name(), state.Version))
	return nil
}
func (s *session) verifyVK(root string) error {
	g, err := gamefiles.Open(root)
	if err != nil {
		return err
	}
	patches, latest, err := catalog.LatestPatches(&http.Client{Timeout: 90 * time.Second}, g.Build)
	if err != nil {
		return fmt.Errorf("update check: %w", err)
	}
	if len(patches) > 0 {
		s.p.Sayf("Update available: %d -> %d. Files are checked against the latest build.\n", g.Build, latest)
		if !s.p.Yes("Install the update first?", true) {
			return errQuit
		}
		if err := gamefiles.InstallPatches(root, patches); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
		if g, err = gamefiles.Open(root); err != nil {
			return err
		}
	}
	s.found.game = &g
	if err := gamefiles.VerifyVKFiles(s.p, g); err != nil {
		return err
	}
	if err := gamefiles.VerifyBeforeLaunch(s.p, g, false); err != nil {
		return err
	}
	s.p.Notify(fmt.Sprintf("VK Play build %d: all files are checked", g.Build))
	return nil
}
func (s *session) fxAccountFor(branch string) (config.Account, bool) {
	want := branch
	if want == gamefiles.DefaultBranch {
		want = ""
	}
	cfg := s.cfg.Get()
	if a, ok := cfg.Find(cfg.LastUserID); ok && a.IsFX() && strings.EqualFold(a.Branch, want) {
		return a, true
	}
	var fallback config.Account
	found := false
	for _, a := range cfg.Accounts {
		if !a.IsFX() {
			continue
		}
		if strings.EqualFold(a.Branch, want) {
			return a, true
		}
		if !found {
			fallback, found = a, true
		}
	}
	return fallback, found
}
