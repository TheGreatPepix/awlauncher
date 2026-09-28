package launcher

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/fxid"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

func (s *session) gameMenu() {
	cfg := s.cfg.Get()
	root := cfg.Game
	if cfg.Game == "" && cfg.FXGame == "" {
		s.p.Say("No game folder is set yet. It is chosen on the first start.")
	}
	info := describeConfiguredGame(cfg)
	if cfg.Game != "" {
		s.p.Sayf("VK Play folder: %s\n", cfg.Game)
	}
	if cfg.FXGame != "" {
		s.p.Sayf("FX ID folder: %s\n", cfg.FXGame)
	}
	if len(info.Clients) == 0 {
		s.p.Say("  The game is not installed.")
	}
	branches := 0
	for _, c := range info.Clients {
		if c.Kind == gamefiles.KindBranch {
			branches++
			s.p.Sayf("  %s, %s: %s\n", c.Name(), c.Version, c.Dir)
		} else {
			s.p.Sayf("  %s, %s: %s\n", c.Name(), c.Version, c.Dir)
		}
	}
	s.p.Sayf("Downloaded patches: %s. Free space: %s.\n", progress.FormatBytes(info.Downloads), progress.FormatBytes(info.Free))
	s.p.Say("")
	s.p.Say("  v        check and repair the game files")
	s.p.Say("  n        check for game updates")
	s.p.Say("  m        set the VK Play game folder")
	s.p.Say("  f        set the FX ID game folder")
	s.p.Say("  p        set a closed branch folder")
	s.p.Say("  c        delete downloaded patches")
	if len(info.Clients) > branches {
		s.p.Say("  d        remove a main client")
	}
	if branches > 0 {
		s.p.Say("  r        remove a closed branch")
	}
	s.p.Say("  u        uninstall the game")
	if root != "" {
		s.p.Say("  o        open the game folder")
	}
	s.p.Say("  Enter    back")
	var err error
	switch firstWord(s.p.line("> ")) {
	case "v", "м":
		var c gamefiles.Client
		if c, err = s.chooseClient("Client number: "); err == nil {
			err = s.verifyClient(c)
		}
	case "n", "т":
		var c gamefiles.Client
		if c, err = s.chooseClient("Client number: "); err == nil {
			var msg string
			if msg, err = s.updateClient(c); err == nil && msg != "" {
				s.p.Notify(msg)
			}
		}
	case "m", "ь":
		err = s.setMainFolder(false)
	case "f", "а":
		err = s.setMainFolder(true)
	case "p", "з":
		err = s.setBranchFolder()
	case "c", "с":
		err = s.clearDownloads()
	case "r", "к":
		var c gamefiles.Client
		if c, err = s.chooseInstalledBranch(); err == nil {
			err = s.removeBranch(c)
		}
	case "d", "в":
		var c gamefiles.Client
		if c, err = s.chooseMainClient(); err == nil {
			err = s.removeMainClient(c)
		}
	case "u", "г":
		err = s.uninstallGame()
	case "o", "щ":
		if root != "" {
			openInShell(root)
		}
	}
	if err != nil && !errors.Is(err, errQuit) {
		s.p.Say("Error:", err)
	}
}

func (s *session) setMainFolder(fx bool) error {
	if err := gamefiles.EnsureGameClosed(); err != nil {
		return err
	}
	cfg := s.cfg.Get()
	label, current := "VK Play", cfg.Game
	if fx {
		label, current = "FX ID", cfg.FXGame
	}
	if current == "" {
		current = platform.DefaultGameDir()
		if fx {
			current += " FX ID"
		}
	}
	dir := strings.Trim(s.p.line(fmt.Sprintf("%s game folder [%s]: ", label, current)), `"' `)
	if dir == "" {
		dir = current
	}
	var err error
	dir, err = filepath.Abs(dir)
	if err != nil {
		return err
	}
	if fx {
		if strings.EqualFold(dir, cfg.Game) || gamefiles.IsVKInstall(dir) {
			return errors.New("choose a folder different from the VK Play client")
		}
		if state, ok := gamefiles.ReadBranchState(dir); ok && state.Branch != gamefiles.DefaultBranch {
			return errors.New("that folder contains a closed FX ID branch")
		}
	} else {
		if strings.EqualFold(dir, cfg.FXGame) {
			return errors.New("choose a folder different from the FX ID client")
		}
		if _, ok := gamefiles.ReadBranchState(dir); ok {
			return errors.New("that folder contains an FX ID client")
		}
	}
	for _, branchDir := range cfg.BranchGames {
		if strings.EqualFold(dir, branchDir) {
			return errors.New("that folder is assigned to a closed branch")
		}
	}
	if err := s.cfg.Update(func(c *config.Config) {
		if fx {
			c.FXGame = dir
		} else {
			c.Game = dir
		}
	}); err != nil {
		return err
	}
	s.found.game = nil
	s.p.Sayf("%s game folder: %s\n", label, dir)
	return nil
}

func (s *session) setBranchFolder() error {
	branch := strings.TrimSpace(s.p.line("FX ID branch name: "))
	if !fxid.ValidBranchName(branch) || strings.EqualFold(branch, gamefiles.DefaultBranch) {
		return errors.New("enter a valid closed branch name")
	}
	current := s.cfg.Get().BranchDir(branch)
	dir := strings.Trim(s.p.line(fmt.Sprintf("Folder [%s]: ", current)), `"' `)
	if dir == "" {
		dir = current
	}
	var err error
	dir, err = filepath.Abs(dir)
	if err != nil {
		return err
	}
	cfg := s.cfg.Get()
	if strings.EqualFold(dir, cfg.Game) || gamefiles.IsVKInstall(dir) {
		return errors.New("choose a folder different from the VK Play client")
	}
	if strings.EqualFold(dir, cfg.FXGame) {
		return errors.New("choose a folder different from the FX ID main client")
	}
	for other, saved := range cfg.BranchGames {
		if !strings.EqualFold(other, branch) && strings.EqualFold(dir, saved) {
			return errors.New("that folder is assigned to another branch")
		}
	}
	if state, ok := gamefiles.ReadBranchState(dir); ok && !strings.EqualFold(state.Branch, branch) {
		return errors.New("that folder contains a different FX ID branch")
	}
	if err := s.cfg.Update(func(c *config.Config) {
		if c.BranchGames == nil {
			c.BranchGames = map[string]string{}
		}
		c.BranchGames[strings.ToLower(branch)] = dir
	}); err != nil {
		return err
	}
	s.p.Sayf("FX ID %s folder: %s\n", branch, dir)
	return nil
}

func (s *session) chooseInstalledBranch() (gamefiles.Client, error) {
	var branches []gamefiles.Client
	for _, c := range installedConfiguredClients(s.cfg.Get()) {
		if c.Kind == gamefiles.KindBranch {
			branches = append(branches, c)
		}
	}
	switch len(branches) {
	case 0:
		return gamefiles.Client{}, errors.New("no closed branches are installed")
	case 1:
		return branches[0], nil
	}
	for i, c := range branches {
		s.p.Sayf("  %d  %s, %s\n", i+1, c.Name(), c.Version)
	}
	n, err := strconv.Atoi(firstWord(s.p.line("Branch number: ")))
	if err != nil || n < 1 || n > len(branches) {
		return gamefiles.Client{}, errQuit
	}
	return branches[n-1], nil
}
