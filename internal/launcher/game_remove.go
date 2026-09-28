package launcher

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cachefiles "github.com/TheGreatPepix/awlauncher/internal/launcher/cache"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

func (s *session) clearDownloads(roots ...string) error {
	if len(roots) == 0 {
		roots = configuredRoots(s.cfg.Get())
	}
	var size int64
	var dirs []string
	for _, root := range roots {
		for _, dir := range gamefiles.DownloadDirs(root) {
			if n := cachefiles.DirSize(dir); n > 0 {
				size += n
				dirs = append(dirs, dir)
			}
		}
	}
	if size == 0 {
		s.p.Say("There are no downloaded patch files.")
		return nil
	}
	if !s.p.Yes(fmt.Sprintf("Delete %s of downloaded patches and backups of replaced files?", progress.FormatBytes(size)), true) {
		return errQuit
	}
	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	s.p.Notify(fmt.Sprintf("Downloaded patches are deleted, %s freed", progress.FormatBytes(size)))
	return nil
}
func (s *session) removeBranch(c gamefiles.Client) error {
	if c.Kind != gamefiles.KindBranch {
		return errors.New("only a closed branch can be removed on its own")
	}
	if err := gamefiles.EnsureGameClosed(); err != nil {
		return err
	}
	if !s.p.Yes(fmt.Sprintf("Remove FX ID %s %s from %s?", c.Branch, c.Version, c.Dir), false) {
		return errQuit
	}
	if err := s.removeClientDir(c.Dir); err != nil {
		return err
	}
	s.p.Sayf("FX ID %s is removed.\n", c.Branch)
	return nil
}
func (s *session) removeMainClient(c gamefiles.Client) error {
	if c.Kind != gamefiles.KindVK && c.Kind != gamefiles.KindFX {
		return errors.New("only a main client can be removed here")
	}
	if err := gamefiles.EnsureGameClosed(); err != nil {
		return err
	}
	installed := false
	for _, other := range installedConfiguredClients(s.cfg.Get()) {
		if other.Kind == c.Kind && strings.EqualFold(filepath.Clean(other.Dir), filepath.Clean(c.Dir)) {
			installed = true
		}
	}
	if !installed {
		return errors.New("the client is no longer installed")
	}
	if !s.p.Yes(fmt.Sprintf("Remove %s from %s?", c.Name(), c.Dir), false) {
		return errQuit
	}
	if err := s.removeClientDir(c.Dir); err != nil {
		return err
	}
	if c.Kind == gamefiles.KindVK {
		s.found.game = nil
	}
	s.p.Sayf("Removed %s.\n", c.Name())
	return nil
}

func (s *session) uninstallGame() error {
	if err := gamefiles.EnsureGameClosed(); err != nil {
		return err
	}
	clients := installedConfiguredClients(s.cfg.Get())
	if len(clients) == 0 {
		return errors.New("no game clients are installed")
	}
	var total int64
	var dirs []string
	seen := map[string]bool{}
	for _, c := range clients {
		key := strings.ToLower(filepath.Clean(c.Dir))
		if seen[key] {
			continue
		}
		seen[key] = true
		dirs = append(dirs, c.Dir)
		total += cachefiles.DirSize(c.Dir)
		s.p.Sayf("  %s, %s: %s\n", c.Name(), c.Version, c.Dir)
	}
	if !s.p.Yes(fmt.Sprintf("Uninstall Armored Warfare and free about %s? Your accounts stay in the launcher.", progress.FormatBytes(total)), false) {
		return errQuit
	}
	for _, dir := range dirs {
		if err := s.removeClientDir(dir); err != nil {
			return err
		}
	}
	s.found.game = nil
	if err := s.cfg.Update(func(c *config.Config) {
		c.Game = ""
		c.FXGame = ""
		c.BranchGames = nil
	}); err != nil {
		return err
	}
	s.p.Say("Armored Warfare is uninstalled.")
	return nil
}
func (s *session) removeClientDir(dir string) error {
	if err := gamefiles.RemoveClientFiles(dir); err != nil {
		return err
	}
	left, size := gamefiles.CountFiles(dir)
	if left == 0 {
		return nil
	}
	s.p.Sayf("%d other files (%s) are left in %s: game settings, logs, screenshots and anything else put there.\n", left, progress.FormatBytes(size), dir)
	if !gamefiles.RemovableFolder(dir) {
		return nil
	}
	if s.p.Yes("Delete the folder with everything in it?", false) {
		return os.RemoveAll(dir)
	}
	return nil
}
