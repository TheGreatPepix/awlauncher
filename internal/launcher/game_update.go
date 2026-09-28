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

func (s *session) updateClient(c gamefiles.Client) (string, error) {
	if c.Kind == gamefiles.KindVK {
		return s.updateVK(c.Dir)
	}
	acc, ok := s.fxAccountFor(c.Branch)
	if !ok {
		return "", errors.New("add an FX ID account first: updates come from FX ID")
	}
	msg, err := s.updateFX(acc, c)
	if errors.Is(err, errNeedLogin) {
		if acc, err = s.relogin(acc); err != nil {
			return "", err
		}
		msg, err = s.updateFX(acc, c)
	}
	return msg, err
}

func (s *session) updateVK(root string) (string, error) {
	g, err := gamefiles.Open(root)
	if err != nil {
		return "", err
	}
	patches, latest, err := catalog.LatestPatches(&http.Client{Timeout: 90 * time.Second}, g.Build)
	if err != nil {
		return "", fmt.Errorf("update check: %w", err)
	}
	if len(patches) == 0 {
		return fmt.Sprintf("VK Play build %d is up to date", g.Build), nil
	}
	if !s.p.Yes(fmt.Sprintf("Update VK Play from build %d to %d (%d patches)?", g.Build, latest, len(patches)), true) {
		return "", errQuit
	}
	if err := gamefiles.InstallPatches(root, patches); err != nil {
		return "", fmt.Errorf("update failed: %w", err)
	}
	s.found.game = nil
	s.p.Notify(fmt.Sprintf("VK Play is updated to build %d", latest))
	return "", nil
}

func (s *session) updateFX(acc config.Account, c gamefiles.Client) (string, error) {
	tokens, err := fxSession(s.client, acc)
	if err != nil {
		return "", err
	}
	m, _, err := fxid.GetBranchManifest(s.client, tokens.AccessToken, c.Branch)
	if err != nil {
		return "", fmt.Errorf("branch %s: %w", c.Branch, err)
	}
	latest := m.Manifest.Release.BuildVersion
	state, _ := gamefiles.ReadBranchState(c.Dir)
	lc := m.Manifest.LauncherConfiguration
	if !state.Dirty && state.ManifestSHA256 != "" && strings.EqualFold(state.ManifestSHA256, lc.ManifestSHA256) {
		return fmt.Sprintf("%s %s is up to date", c.Name(), state.Version), nil
	}
	if !s.p.Yes(fmt.Sprintf("Update %s from %s to %s?", c.Name(), c.Version, latest), true) {
		return "", errQuit
	}
	if err := gamefiles.EnsureGameClosed(); err != nil {
		return "", err
	}
	mainRoot := ""
	if c.Kind == gamefiles.KindBranch {
		mainRoot = s.cfg.Get().FXGame
	}
	if _, err := syncFXClient(s.client, &s.p, acc, c.Branch, c.Dir, mainRoot, false, s.cfg.Get().AllowMods); err != nil {
		return "", err
	}
	return "", nil
}
