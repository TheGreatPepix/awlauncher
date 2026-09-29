package launcher

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/vkplay"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
)

func (s *Session) playVK(acc config.Account) error {
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
	s.ui.Notify(fmt.Sprintf("VK Play %s is installed", gamefiles.VKLabel(gamefiles.VKVersion(g.Root), g.Build)))
	return g, true
}

func (s *Session) ensureUpdated(g *gamefiles.Install) bool {
	s.ui.Say("Checking for updates...")
	patches, latest, err := vkplay.LatestPatches(&http.Client{Timeout: 90 * time.Second}, g.Build)
	if err != nil {
		s.ui.Say("Update check failed:", err)
		return s.ui.Yes("Continue anyway?", false)
	}
	if len(patches) == 0 {
		gamefiles.CleanupCache(gamefiles.CacheDir(g.Root), g.Build)
		s.ui.Sayf("VK Play %s is up to date.", gamefiles.VKLabel(gamefiles.VKVersion(g.Root), g.Build))
		return true
	}
	s.ui.Sayf("Update available: %s -> %s (%d patches).", gamefiles.VKLabel(gamefiles.VKVersion(g.Root), g.Build), s.vkLatestLabel(latest), len(patches))
	if !s.ui.Yes("Install now?", true) {
		return s.ui.Yes("Start the game without updating?", false)
	}
	if err := gamefiles.InstallPatches(g.Root, patches, !s.cfg.Get().NoBackups); err != nil {
		s.ui.Say("Update failed:", err)
		return false
	}
	updated, err := gamefiles.Open(g.Root)
	if err != nil {
		s.ui.Say("After update:", err)
		return false
	}
	s.ui.Notify(fmt.Sprintf("VK Play is updated to %s", gamefiles.VKLabel(gamefiles.VKVersion(updated.Root), updated.Build)))
	*g = updated
	return true
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

func (s *Session) startVK(g gamefiles.Install, acc config.Account) error {
	if name, err := platform.RunningProcess(platform.GameExe); err != nil {
		return err
	} else if name != "" {
		return errors.New("the game is already running")
	}
	ticket, err := vkplay.Ticket(s.client, acc.UserID)
	if err != nil {
		return err
	}
	pid, err := platform.StartGame(g.Exe, g.Launch.Args(ticket.GameAccount, ticket.Code), g.Root, vkplay.GameEnv(ticket.GameAccount))
	if err != nil {
		return fmt.Errorf("start the game: %w", err)
	}
	s.ui.Sayf("Game started: %s, PID %d", acc.Label(), pid)
	return nil
}

func (s *Session) downloadVK() error {
	root, err := s.vkRoot()
	if err != nil {
		return err
	}
	if err := gamefiles.InstallVK(s.ui, root); err != nil {
		return err
	}
	s.ui.Notify("VK Play is installed")
	return nil
}

func (s *Session) updateVK(root string) error {
	g, err := gamefiles.Open(root)
	if err != nil {
		return err
	}
	patches, latest, err := vkplay.LatestPatches(&http.Client{Timeout: 90 * time.Second}, g.Build)
	if err != nil {
		return fmt.Errorf("update check: %w", err)
	}
	if len(patches) == 0 {
		s.ui.Notify(fmt.Sprintf("VK Play %s is up to date", gamefiles.VKLabel(gamefiles.VKVersion(g.Root), g.Build)))
		return nil
	}
	if !s.ui.Yes(fmt.Sprintf("Update VK Play from %s to %s?", gamefiles.VKLabel(gamefiles.VKVersion(g.Root), g.Build), s.vkLatestLabel(latest)), true) {
		return ErrCancelled
	}
	if err := gamefiles.InstallPatches(root, patches, !s.cfg.Get().NoBackups); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	s.found.game = nil
	s.ui.Notify(fmt.Sprintf("VK Play is updated to %s", gamefiles.VKLabel(gamefiles.VKVersion(root), latest)))
	return nil
}

func (s *Session) verifyVK(root string) error {
	g, err := gamefiles.Open(root)
	if err != nil {
		return err
	}
	patches, latest, err := vkplay.LatestPatches(&http.Client{Timeout: 90 * time.Second}, g.Build)
	if err != nil {
		return fmt.Errorf("update check: %w", err)
	}
	if len(patches) > 0 {
		s.ui.Sayf("Update available: %s -> %s. Files are checked against the latest build.", gamefiles.VKLabel(gamefiles.VKVersion(g.Root), g.Build), s.vkLatestLabel(latest))
		if !s.ui.Yes("Install the update first?", true) {
			return ErrCancelled
		}
		if err := gamefiles.InstallPatches(root, patches, !s.cfg.Get().NoBackups); err != nil {
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
	s.ui.Notify(fmt.Sprintf("VK Play %s: all files are checked", gamefiles.VKLabel(gamefiles.VKVersion(g.Root), g.Build)))
	return nil
}

func (s *Session) vkLatestLabel(build int) string {
	return gamefiles.VKLabel(vkplay.BuildVersion(&http.Client{Timeout: 30 * time.Second}, build), build)
}
