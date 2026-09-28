package launcher

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

func (s *Session) ClearDownloads(roots ...string) error {
	if len(roots) == 0 {
		roots = configuredRoots(s.cfg.Get())
	}
	var size int64
	var dirs []string
	for _, root := range roots {
		for _, dir := range gamefiles.DownloadDirs(root) {
			if n := gamefiles.DirSize(dir); n > 0 {
				size += n
				dirs = append(dirs, dir)
			}
		}
	}
	if size == 0 {
		s.ui.Say("There are no downloaded patch files.")
		return nil
	}
	if !s.ui.Yes(fmt.Sprintf("Delete %s of downloaded patches and backups of replaced files?", progress.FormatBytes(size)), true) {
		return ErrCancelled
	}
	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	s.ui.Notify(fmt.Sprintf("Downloaded patches are deleted, %s freed", progress.FormatBytes(size)))
	return nil
}
func (s *Session) RemoveBranch(c gamefiles.Client) error {
	if c.Kind != gamefiles.KindBranch {
		return errors.New("only a closed branch can be removed on its own")
	}
	if err := gamefiles.EnsureGameClosed(); err != nil {
		return err
	}
	if !s.ui.Yes(fmt.Sprintf("Remove FX ID %s %s from %s?", c.Branch, c.Version, c.Dir), false) {
		return ErrCancelled
	}
	if err := s.removeClientDir(c.Dir); err != nil {
		return err
	}
	s.ui.Sayf("FX ID %s is removed.", c.Branch)
	return nil
}
func (s *Session) RemoveMainClient(c gamefiles.Client) error {
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
	if !s.ui.Yes(fmt.Sprintf("Remove %s from %s?", c.Name(), c.Dir), false) {
		return ErrCancelled
	}
	if err := s.removeClientDir(c.Dir); err != nil {
		return err
	}
	if c.Kind == gamefiles.KindVK {
		s.found.game = nil
	}
	s.ui.Sayf("Removed %s.", c.Name())
	return nil
}

func (s *Session) removeClientDir(dir string) error {
	if err := gamefiles.RemoveClientFiles(dir); err != nil {
		return err
	}
	left, size := gamefiles.CountFiles(dir)
	if left == 0 {
		return nil
	}
	s.ui.Sayf("%d other files (%s) are left in %s: game settings, logs, screenshots and anything else put there.", left, progress.FormatBytes(size), dir)
	if !gamefiles.RemovableFolder(dir) {
		return nil
	}
	if s.ui.Yes("Delete the folder with everything in it?", false) {
		return os.RemoveAll(dir)
	}
	return nil
}
