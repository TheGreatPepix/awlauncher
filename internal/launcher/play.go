package launcher

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/cache"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/catalog"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
)

func (s *Session) Play(acc config.Account) error {
	return s.withLogin(acc, func(acc config.Account) error {
		if err := s.play(acc); err != nil {
			return err
		}
		return s.Pin(acc)
	})
}

func (s *Session) play(acc config.Account) error {
	switch {
	case acc.IsFX() && acc.Branch != "":
		cfg := s.cfg.Get()
		if cfg.FXGame == "" && cfg.BranchGames[strings.ToLower(acc.Branch)] == "" {
			if _, err := s.fxRoot(); err != nil {
				return err
			}
			cfg = s.cfg.Get()
		}
		return s.playFX(acc, acc.Branch, cfg.BranchDir(acc.Branch), cfg.FXGame, false)
	case acc.IsFX():
		root, err := s.fxRoot()
		if err != nil {
			return err
		}
		return s.playFX(acc, gamefiles.DefaultBranch, root, "", true)
	}
	g, err := s.vkGame()
	if err != nil {
		return err
	}
	if err := gamefiles.VerifyBeforeLaunch(s.ui, g, s.cfg.Get().AllowMods); err != nil {
		return err
	}
	return s.startVK(g, acc)
}

func (s *Session) vkGame() (gamefiles.Install, error) {
	if s.found.game == nil {
		g, err := s.chooseGameDir()
		if err != nil {
			return gamefiles.Install{}, err
		}
		if s.cfg.Get().Game != g.Root {
			if err := s.cfg.Update(func(c *config.Config) { c.Game = g.Root }); err != nil {
				return gamefiles.Install{}, err
			}
		}
		s.ui.Sayf("Game: %s (build %d)", g.Root, g.Build)
		if !s.ensureUpdated(&g) {
			return gamefiles.Install{}, ErrCancelled
		}
		s.found.game = &g
	}
	return *s.found.game, nil
}

func (s *Session) chooseGameDir() (gamefiles.Install, error) {
	if saved := s.cfg.Get().Game; saved != "" {
		g, err := gamefiles.Open(saved)
		if err == nil {
			return g, nil
		}
		if !gamefiles.IsVKInstall(saved) && s.ui.Yes(fmt.Sprintf("The game is not installed in %s (unfinished install?). Install it there?", saved), true) {
			if g, ok := s.installInto(saved); ok {
				return g, nil
			}
		} else {
			s.ui.Sayf("Saved game folder is not usable (%v).", err)
		}
	}
	detected := gamefiles.DetectVKInstall()
	if detected != "" && s.ui.Yes(fmt.Sprintf("Found the game in %s. Use it?", detected), true) {
		g, err := gamefiles.Open(detected)
		if err == nil {
			return g, nil
		}
		s.ui.Say("Not usable:", err)
	}
	if detected == "" {
		s.ui.Say("Armored Warfare was not found. Enter the folder of an existing install, or any folder to install the game into.")
	}
	for {
		dir, ok := s.askFolder(gamefiles.KindVK)
		if !ok {
			return gamefiles.Install{}, ErrCancelled
		}
		g, err := gamefiles.Open(dir)
		if err == nil {
			return g, nil
		}
		if gamefiles.IsVKInstall(dir) {
			s.ui.Say("Not usable:", err)
			continue
		}
		if !s.ui.Yes(fmt.Sprintf("There is no Armored Warfare install in %s. Install the game there?", dir), true) {
			continue
		}
		if g, ok := s.installInto(dir); ok {
			return g, nil
		}
	}
}

func (s *Session) installInto(dir string) (gamefiles.Install, bool) {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if _, ok := gamefiles.ReadBranchState(dir); ok || strings.EqualFold(dir, s.cfg.Get().FXGame) {
		s.ui.Say("Cannot install VK Play over an FX ID client.")
		return gamefiles.Install{}, false
	}
	var previous string
	if err := s.cfg.Update(func(c *config.Config) { previous, c.Game = c.Game, dir }); err != nil {
		s.ui.Say("Error:", err)
	}
	if err := gamefiles.InstallVK(s.ui, dir); err != nil {
		if errors.Is(err, gamefiles.ErrInstallCancelled) {
			_ = s.cfg.Update(func(c *config.Config) {
				if c.Game == dir {
					c.Game = previous
				}
			})
		} else {
			s.ui.Say("Install failed:", err)
		}
		return gamefiles.Install{}, false
	}
	g, err := gamefiles.Open(dir)
	if err != nil {
		s.ui.Say("Not usable after install:", err)
		return gamefiles.Install{}, false
	}
	s.ui.Notify(fmt.Sprintf("VK Play build %d is installed", g.Build))
	return g, true
}

func (s *Session) ensureUpdated(g *gamefiles.Install) bool {
	s.ui.Say("Checking for updates...")
	patches, latest, err := catalog.LatestPatches(&http.Client{Timeout: 90 * time.Second}, g.Build)
	if err != nil {
		s.ui.Say("Update check failed:", err)
		return s.ui.Yes("Continue anyway?", false)
	}
	if len(patches) == 0 {
		cache.Cleanup(gamefiles.CacheDir(g.Root), g.Build)
		s.ui.Sayf("Build %d is up to date.", g.Build)
		return true
	}
	s.ui.Sayf("Update available: %d -> %d (%d patches).", g.Build, latest, len(patches))
	if !s.ui.Yes("Install now?", true) {
		return s.ui.Yes("Start the game without updating?", false)
	}
	if err := gamefiles.InstallPatches(g.Root, patches); err != nil {
		s.ui.Say("Update failed:", err)
		return false
	}
	updated, err := gamefiles.Open(g.Root)
	if err != nil {
		s.ui.Say("After update:", err)
		return false
	}
	s.ui.Notify(fmt.Sprintf("VK Play is updated to build %d", updated.Build))
	*g = updated
	return true
}

func (s *Session) askFolder(kind string) (string, bool) {
	answer, ok := s.ui.Ask(s.FolderPrompt(kind, ""))
	dir := strings.Trim(strings.TrimSpace(answer), `"'`)
	if !ok || dir == "" {
		return "", false
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	return dir, true
}

func (s *Session) vkRoot() (string, error) {
	cfg := s.cfg.Get()
	if cfg.Game != "" {
		return cfg.Game, nil
	}
	for {
		root, ok := s.askFolder(gamefiles.KindVK)
		if !ok {
			return "", ErrCancelled
		}
		if _, ok := gamefiles.ReadBranchState(root); ok || strings.EqualFold(root, cfg.FXGame) {
			s.ui.Say("That folder contains an FX ID client. Choose another folder.")
			continue
		}
		if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 && !gamefiles.IsVKInstall(root) && !s.ui.Yes("The folder is not empty. Continue there?", false) {
			continue
		}
		if err := s.cfg.Update(func(c *config.Config) { c.Game = root }); err != nil {
			return "", err
		}
		return root, nil
	}
}

func (s *Session) fxRoot() (string, error) {
	cfg := s.cfg.Get()
	if cfg.FXGame != "" {
		return cfg.FXGame, nil
	}
	for {
		root, ok := s.askFolder(gamefiles.KindFX)
		if !ok {
			return "", ErrCancelled
		}
		if strings.EqualFold(root, cfg.Game) || gamefiles.IsVKInstall(root) {
			s.ui.Say("Choose a folder different from the VK Play client.")
			continue
		}
		if state, ok := gamefiles.ReadBranchState(root); ok && state.Branch != gamefiles.DefaultBranch {
			s.ui.Say("That folder contains a closed FX ID branch.")
			continue
		}
		if err := s.cfg.Update(func(c *config.Config) { c.FXGame = root }); err != nil {
			return "", err
		}
		return root, nil
	}
}
