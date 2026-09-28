package launcher

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/catalog"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
)

func (s *Session) FindClient(kind, branch string) (gamefiles.Client, error) {
	root := s.cfg.Get().Game
	for _, c := range installedConfiguredClients(s.cfg.Get()) {
		if c.Kind == kind && (kind != gamefiles.KindBranch || strings.EqualFold(c.Branch, branch)) {
			return c, nil
		}
	}
	return gamefiles.Client{}, fmt.Errorf("that client is not installed in %s", root)
}
func (s *Session) VerifyClient(c gamefiles.Client) error {
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
	var state gamefiles.BranchState
	err := s.withLogin(acc, func(acc config.Account) error {
		var err error
		state, err = s.syncFX(acc, c.Branch, c.Dir, root, false, false)
		return err
	})
	if err != nil {
		return err
	}
	s.ui.Notify(fmt.Sprintf("%s %s: all files are checked", c.Name(), state.Version))
	return nil
}
func (s *Session) verifyVK(root string) error {
	g, err := gamefiles.Open(root)
	if err != nil {
		return err
	}
	patches, latest, err := catalog.LatestPatches(&http.Client{Timeout: 90 * time.Second}, g.Build)
	if err != nil {
		return fmt.Errorf("update check: %w", err)
	}
	if len(patches) > 0 {
		s.ui.Sayf("Update available: %d -> %d. Files are checked against the latest build.", g.Build, latest)
		if !s.ui.Yes("Install the update first?", true) {
			return ErrCancelled
		}
		if err := gamefiles.InstallPatches(root, patches); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
		if g, err = gamefiles.Open(root); err != nil {
			return err
		}
	}
	s.found.game = &g
	if err := gamefiles.VerifyVKFiles(s.ui, g); err != nil {
		return err
	}
	if err := gamefiles.VerifyBeforeLaunch(s.ui, g, false); err != nil {
		return err
	}
	s.ui.Notify(fmt.Sprintf("VK Play build %d: all files are checked", g.Build))
	return nil
}
func (s *Session) fxAccountFor(branch string) (config.Account, bool) {
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
