package launcher

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/fxid"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

func playFXInstall(client *http.Client, p *prompter, acc config.Account, branch, root, mainRoot string, ask, allowMods bool) error {
	state, err := syncFXClient(client, p, acc, branch, root, mainRoot, ask, allowMods)
	if err != nil {
		return err
	}
	launchToken, err := fxGameToken(client, acc)
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
	pid, err := platform.StartGame(exe, fxLaunchArgs(client, acc, launchToken), root)
	if err != nil {
		return fmt.Errorf("start the game: %w", err)
	}
	p.Sayf("Game started: %s, branch %s %s, PID %d\n", acc.Label(), branch, state.Version, pid)
	return nil
}

func syncFXClient(client *http.Client, p *prompter, acc config.Account, branch, root, mainRoot string, ask, allowMods bool) (gamefiles.BranchState, error) {
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
	tokens, err := fxSession(client, acc)
	if err != nil {
		return gamefiles.BranchState{}, err
	}
	m, _, err := fxid.GetBranchManifest(client, tokens.AccessToken, branch)
	if err != nil {
		return gamefiles.BranchState{}, fmt.Errorf("branch %s: %w", branch, err)
	}
	if ask {
		if state, ok := gamefiles.ReadBranchState(root); !ok || state.Branch != branch {
			if !p.Yes(fmt.Sprintf("Download the FX ID client (%s) into %s?", progress.FormatBytes(m.FullSize()), root), false) {
				return gamefiles.BranchState{}, errQuit
			}
		}
	}
	if strings.EqualFold(filepath.Clean(mainRoot), filepath.Clean(root)) {
		mainRoot = ""
	}
	prev, had := gamefiles.ReadBranchState(root)
	state, err := gamefiles.SyncBranch(client, branchRelease(branch, m), mainRoot, root, allowMods)
	if err != nil {
		return state, err
	}
	name := gamefiles.Client{Kind: gamefiles.KindBranch, Branch: branch}.Name()
	if branch == gamefiles.DefaultBranch {
		name = gamefiles.Client{Kind: gamefiles.KindFX}.Name()
	}
	switch {
	case !had:
		p.Notify(fmt.Sprintf("%s %s is installed", name, state.Version))
	case prev.Version != state.Version:
		p.Notify(fmt.Sprintf("%s is updated to %s", name, state.Version))
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
