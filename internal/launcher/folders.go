package launcher

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/fxid"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
)

var errNotAFolder = errors.New("Enter a full path on an existing drive, like D:\\Games\\Armored Warfare.")

type FolderInfo struct {
	Path    string `json:"path"`
	Valid   bool   `json:"valid"`
	Free    int64  `json:"free"`
	Drive   string `json:"drive"`
	Install bool   `json:"install"`
	Branch  string `json:"branch"`
	Used    bool   `json:"used"`
}

func DescribeFolder(dir string) FolderInfo {
	info := FolderInfo{Path: dir}
	dir = strings.Trim(strings.TrimSpace(dir), `"'`)
	if dir == "" || !filepath.IsAbs(dir) {
		return info
	}
	info.Drive = filepath.VolumeName(dir)
	if _, err := os.Stat(info.Drive + string(filepath.Separator)); err != nil {
		return info
	}
	info.Valid = true
	info.Free, _ = platform.DiskFree(dir)
	if state, ok := gamefiles.ReadBranchState(dir); ok {
		if state.Branch == gamefiles.DefaultBranch {
			info.Install = true
		} else {
			info.Branch = state.Branch
		}
	}
	info.Install = info.Install || gamefiles.IsVKInstall(dir)
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		info.Used = true
	}
	return info
}

func SuggestGameFolder(saved string) string {
	if saved != "" {
		return saved
	}
	return platform.DefaultGameDir()
}

func (s *Session) FolderPrompt(kind, branch string) Prompt {
	cfg := s.cfg.Get()
	p := Prompt{Kind: PromptFolder, Folder: kind, Branch: branch}
	switch kind {
	case gamefiles.KindFX:
		p.Question, p.Suggest = "FX ID game folder", cfg.FXGame
		if p.Suggest == "" {
			p.Suggest = config.DefaultFXDir()
		}
	case gamefiles.KindBranch:
		p.Question, p.Suggest = "Game folder for FX ID "+branch, cfg.BranchDir(branch)
	default:
		p.Question, p.Suggest = "VK Play game folder", SuggestGameFolder(cfg.Game)
	}
	return p
}

func cleanFolder(dir string) (string, error) {
	dir = strings.Trim(strings.TrimSpace(dir), `"'`)
	if !DescribeFolder(dir).Valid {
		return "", errNotAFolder
	}
	return filepath.Clean(dir), nil
}

func (s *Session) SetVKFolder(dir string) error {
	dir, err := cleanFolder(dir)
	if err != nil {
		return err
	}
	cfg := s.cfg.Get()
	if _, ok := gamefiles.ReadBranchState(dir); ok || strings.EqualFold(dir, cfg.FXGame) {
		return errors.New("That folder contains an FX ID client. Choose another folder.")
	}
	for _, branchDir := range cfg.BranchGames {
		if strings.EqualFold(dir, branchDir) {
			return errors.New("That folder is assigned to a closed branch.")
		}
	}
	if err := s.cfg.Update(func(c *config.Config) { c.Game = dir }); err != nil {
		return err
	}
	s.found.game = nil
	s.ui.Say("VK Play game folder:", dir)
	return nil
}

func (s *Session) SetFXFolder(dir string) error {
	dir, err := cleanFolder(dir)
	if err != nil {
		return err
	}
	cfg := s.cfg.Get()
	if strings.EqualFold(dir, cfg.Game) || gamefiles.IsVKInstall(dir) {
		return errors.New("Choose a folder different from the VK Play client.")
	}
	for _, branchDir := range cfg.BranchGames {
		if strings.EqualFold(dir, branchDir) {
			return errors.New("That folder is assigned to a closed branch.")
		}
	}
	if state, ok := gamefiles.ReadBranchState(dir); ok && state.Branch != gamefiles.DefaultBranch {
		return errors.New("That folder contains a closed FX ID branch.")
	}
	if err := s.cfg.Update(func(c *config.Config) { c.FXGame = dir }); err != nil {
		return err
	}
	s.ui.Say("FX ID game folder:", dir)
	return nil
}

func (s *Session) SetBranchFolder(branch, dir string) error {
	if !fxid.ValidBranchName(branch) || strings.EqualFold(branch, gamefiles.DefaultBranch) {
		return errors.New("Enter a valid closed branch name.")
	}
	cfg := s.cfg.Get()
	if strings.TrimSpace(dir) == "" {
		dir = cfg.BranchDir(branch)
	}
	dir, err := cleanFolder(dir)
	if err != nil {
		return err
	}
	if strings.EqualFold(dir, cfg.Game) {
		return errors.New("Choose a folder different from the VK Play folder.")
	}
	if strings.EqualFold(dir, cfg.FXGame) {
		return errors.New("Choose a folder different from the FX ID main client.")
	}
	for other, saved := range cfg.BranchGames {
		if !strings.EqualFold(other, branch) && strings.EqualFold(dir, saved) {
			return errors.New("That folder is assigned to another branch.")
		}
	}
	if state, ok := gamefiles.ReadBranchState(dir); ok && !strings.EqualFold(state.Branch, branch) {
		return errors.New("That folder contains a different FX ID branch.")
	}
	if gamefiles.IsVKInstall(dir) {
		return errors.New("That folder contains the VK Play client.")
	}
	if err := s.cfg.Update(func(c *config.Config) {
		if c.BranchGames == nil {
			c.BranchGames = map[string]string{}
		}
		c.BranchGames[strings.ToLower(branch)] = dir
	}); err != nil {
		return err
	}
	s.ui.Sayf("FX ID %s folder: %s", branch, dir)
	return nil
}
