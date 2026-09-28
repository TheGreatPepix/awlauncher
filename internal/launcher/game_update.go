package launcher

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/catalog"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/fxid"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
)

func (s *Session) UpdateClient(c gamefiles.Client) error {
	if c.Kind == gamefiles.KindVK {
		return s.updateVK(c.Dir)
	}
	acc, ok := s.fxAccountFor(c.Branch)
	if !ok {
		return errors.New("add an FX ID account first: updates come from FX ID")
	}
	return s.withLogin(acc, func(acc config.Account) error { return s.updateFX(acc, c) })
}

func (s *Session) updateVK(root string) error {
	g, err := gamefiles.Open(root)
	if err != nil {
		return err
	}
	patches, latest, err := catalog.LatestPatches(&http.Client{Timeout: 90 * time.Second}, g.Build)
	if err != nil {
		return fmt.Errorf("update check: %w", err)
	}
	if len(patches) == 0 {
		s.ui.Notify(fmt.Sprintf("VK Play build %d is up to date", g.Build))
		return nil
	}
	if !s.ui.Yes(fmt.Sprintf("Update VK Play from build %d to %d (%d patches)?", g.Build, latest, len(patches)), true) {
		return ErrCancelled
	}
	if err := gamefiles.InstallPatches(root, patches); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	s.found.game = nil
	s.ui.Notify(fmt.Sprintf("VK Play is updated to build %d", latest))
	return nil
}

func (s *Session) updateFX(acc config.Account, c gamefiles.Client) error {
	tokens, err := fxSession(s.client, acc)
	if err != nil {
		return err
	}
	m, _, err := fxid.GetBranchManifest(s.client, tokens.AccessToken, c.Branch)
	if err != nil {
		return fmt.Errorf("branch %s: %w", c.Branch, err)
	}
	latest := m.Manifest.Release.BuildVersion
	state, _ := gamefiles.ReadBranchState(c.Dir)
	lc := m.Manifest.LauncherConfiguration
	if !state.Dirty && state.ManifestSHA256 != "" && strings.EqualFold(state.ManifestSHA256, lc.ManifestSHA256) {
		s.ui.Notify(fmt.Sprintf("%s %s is up to date", c.Name(), state.Version))
		return nil
	}
	if !s.ui.Yes(fmt.Sprintf("Update %s from %s to %s?", c.Name(), c.Version, latest), true) {
		return ErrCancelled
	}
	if err := gamefiles.EnsureGameClosed(); err != nil {
		return err
	}
	mainRoot := ""
	if c.Kind == gamefiles.KindBranch {
		mainRoot = s.cfg.Get().FXGame
	}
	_, err = s.syncFX(acc, c.Branch, c.Dir, mainRoot, false, s.cfg.Get().AllowMods)
	return err
}
