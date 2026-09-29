package launcher

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/fxid"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

func (s *Session) playFX(acc config.Account) error {
	if acc.Branch != "" {
		cfg := s.cfg.Get()
		if cfg.FXGame == "" && cfg.BranchGames[strings.ToLower(acc.Branch)] == "" {
			if _, err := s.fxRoot(); err != nil {
				return err
			}
			cfg = s.cfg.Get()
		}
		return s.launchFX(acc, acc.Branch, cfg.BranchDir(acc.Branch), cfg.FXGame, false)
	}
	root, err := s.fxRoot()
	if err != nil {
		return err
	}
	return s.launchFX(acc, gamefiles.DefaultBranch, root, "", true)
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

func (s *Session) launchFX(acc config.Account, branch, root, mainRoot string, ask bool) error {
	state, err := s.syncFX(acc, branch, root, mainRoot, ask, s.cfg.Get().AllowMods)
	if err != nil {
		return err
	}
	launchToken, err := fxid.GameToken(s.client, acc.UserID, platform.Language())
	if err != nil {
		return err
	}
	launch := state.LaunchFile
	if launch == "" {
		launch = "bin64/ArmoredWarfare.exe"
	}
	exe, err := gamefiles.SafePath(root, launch)
	if err != nil {
		return err
	}
	pid, err := platform.StartGame(exe, fxLaunchArgs(s.client, acc, launchToken), root)
	if err != nil {
		return fmt.Errorf("start the game: %w", err)
	}
	s.ui.Sayf("Game started: %s, branch %s %s, PID %d", acc.Label(), branch, state.Version, pid)
	return nil
}

func (s *Session) syncFX(acc config.Account, branch, root, mainRoot string, ask, allowMods bool) (gamefiles.BranchState, error) {
	if gamefiles.IsVKInstall(root) {
		return gamefiles.BranchState{}, errors.New("FX ID client folder contains a VK Play installation")
	}
	if state, ok := gamefiles.ReadBranchState(root); ok && state.Branch != branch {
		return gamefiles.BranchState{}, fmt.Errorf("FX ID client folder belongs to branch %s", state.Branch)
	}
	if name, err := platform.RunningProcess(platform.GameExe); err != nil {
		return gamefiles.BranchState{}, err
	} else if name != "" {
		return gamefiles.BranchState{}, errors.New("the game is already running")
	}
	session, err := fxid.Session(s.client, acc.UserID, platform.Language())
	if err != nil {
		return gamefiles.BranchState{}, err
	}
	m, _, err := fxid.GetBranchManifest(s.client, session.AccessToken, branch)
	if err != nil {
		return gamefiles.BranchState{}, fmt.Errorf("branch %s: %w", branch, err)
	}
	if ask {
		if state, ok := gamefiles.ReadBranchState(root); !ok || state.Branch != branch {
			if !s.ui.Yes(fmt.Sprintf("Download the FX ID client (%s) into %s?", progress.FormatBytes(m.FullSize()), root), false) {
				return gamefiles.BranchState{}, ErrCancelled
			}
		}
	}
	if strings.EqualFold(filepath.Clean(mainRoot), filepath.Clean(root)) {
		mainRoot = ""
	}
	prev, had := gamefiles.ReadBranchState(root)
	state, err := gamefiles.SyncBranch(s.client, branchRelease(branch, m), mainRoot, root, allowMods)
	if err != nil {
		return state, err
	}
	name := gamefiles.Client{Kind: gamefiles.KindBranch, Branch: branch}.Name()
	if branch == gamefiles.DefaultBranch {
		name = gamefiles.Client{Kind: gamefiles.KindFX}.Name()
	}
	switch {
	case !had:
		s.ui.Notify(fmt.Sprintf("%s %s is installed", name, state.Version))
	case prev.Version != state.Version:
		s.ui.Notify(fmt.Sprintf("%s is updated to %s", name, state.Version))
	}
	return state, nil
}

func branchRelease(branch string, m fxid.BranchManifest) gamefiles.BranchRelease {
	lc := m.Manifest.LauncherConfiguration
	return gamefiles.BranchRelease{
		Name:            branch,
		Version:         m.Manifest.Release.BuildVersion,
		Build:           m.Manifest.Release.BuildNumber,
		ManifestURL:     lc.ManifestURL,
		ManifestSHA256:  lc.ManifestSHA256,
		DownloadBaseURI: lc.HTTPDownload.DownloadBaseURI,
		LaunchFile:      lc.LaunchFile,
	}
}

func (s *Session) downloadFX(acc config.Account, kind, branch string) error {
	if !acc.IsFX() || !fxid.ValidBranchName(branch) {
		return errors.New("an FX ID account and valid branch are required")
	}
	if (kind == gamefiles.KindFX) != strings.EqualFold(branch, gamefiles.DefaultBranch) || (kind != gamefiles.KindFX && kind != gamefiles.KindBranch) {
		return errors.New("invalid client branch")
	}
	branches, err := fxBranches(s.client, acc)
	if err != nil {
		return err
	}
	available := false
	for _, b := range branches {
		if b.Err == nil && strings.EqualFold(b.Name, branch) {
			branch, available = b.Name, true
			break
		}
	}
	if !available {
		return fmt.Errorf("branch %s is not available to this account", branch)
	}
	cfg := s.cfg.Get()
	if kind == gamefiles.KindFX {
		root, err := s.fxRoot()
		if err != nil {
			return err
		}
		_, err = s.syncFX(acc, branch, root, "", true, cfg.AllowMods)
		return err
	}
	if cfg.FXGame == "" && cfg.BranchGames[strings.ToLower(branch)] == "" {
		if _, err := s.fxRoot(); err != nil {
			return err
		}
		cfg = s.cfg.Get()
	}
	_, err = s.syncFX(acc, branch, cfg.BranchDir(branch), cfg.FXGame, true, cfg.AllowMods)
	return err
}

func (s *Session) updateFX(acc config.Account, c gamefiles.Client) error {
	session, err := fxid.Session(s.client, acc.UserID, platform.Language())
	if err != nil {
		return err
	}
	m, _, err := fxid.GetBranchManifest(s.client, session.AccessToken, c.Branch)
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

func (s *Session) verifyFX(c gamefiles.Client) error {
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
