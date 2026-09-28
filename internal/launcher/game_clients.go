package launcher

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	cachefiles "github.com/TheGreatPepix/awlauncher/internal/launcher/cache"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/catalog"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/fxid"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
)

type gameInfo struct {
	Type        string             `json:"type"`
	Dir         string             `json:"dir"`
	Clients     []gamefiles.Client `json:"clients"`
	Downloads   int64              `json:"downloads"`
	Free        int64              `json:"free"`
	BranchPaths map[string]string  `json:"branchPaths,omitempty"`
}
type availableClient struct {
	Kind    string `json:"kind"`
	Branch  string `json:"branch,omitempty"`
	Account string `json:"account"`
	Version string `json:"version,omitempty"`
}
type availableClients struct {
	Type     string            `json:"type"`
	Clients  []availableClient `json:"clients"`
	Failed   bool              `json:"failed,omitempty"`
	VKFailed bool              `json:"vkFailed,omitempty"`
}

func availableGameClients(s *session) availableClients {
	out := availableClients{Type: "availableClients", Clients: []availableClient{}}
	seen := map[string]bool{}
	for _, acc := range s.cfg.Get().Accounts {
		kind := gamefiles.KindVK
		if acc.IsFX() {
			kind = gamefiles.KindFX
		} else {
			if !seen[kind] {
				client := availableClient{Kind: kind, Account: strconv.FormatInt(acc.UserID, 10)}
				if distrib, err := catalog.LatestDistrib(s.client); err == nil {
					client.Version = "build " + strconv.Itoa(distrib.Destination)
				} else {
					out.VKFailed = true
				}
				out.Clients = append(out.Clients, client)
				seen[kind] = true
			}
			continue
		}
		branches, err := fxBranches(s.client, acc)
		if err != nil {
			out.Failed = true
			continue
		}
		for _, b := range branches {
			if b.Err != nil {
				out.Failed = true
				continue
			}
			if !fxid.ValidBranchName(b.Name) {
				continue
			}
			k := strings.ToLower(b.Name)
			if seen[k] {
				continue
			}
			clientKind := gamefiles.KindBranch
			if strings.EqualFold(b.Name, gamefiles.DefaultBranch) {
				clientKind = gamefiles.KindFX
			}
			out.Clients = append(out.Clients, availableClient{Kind: clientKind, Branch: b.Name, Account: strconv.FormatInt(acc.UserID, 10), Version: b.Version})
			seen[k] = true
		}
	}
	return out
}
func (s *session) downloadClient(acc config.Account, kind, branch string) error {
	if err := gamefiles.EnsureGameClosed(); err != nil {
		return err
	}
	if kind == gamefiles.KindVK {
		if acc.IsFX() {
			return errors.New("a VK Play account is required")
		}
		root, err := s.vkRoot()
		if err != nil {
			return err
		}
		if err := gamefiles.InstallVK(s.p, root); err != nil {
			return err
		}
		s.p.Notify("VK Play is installed")
		return nil
	}
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
		_, err = syncFXClient(s.client, &s.p, acc, branch, root, "", true, cfg.AllowMods)
		return err
	}
	if cfg.FXGame == "" && cfg.BranchGames[strings.ToLower(branch)] == "" {
		if _, err := s.fxRoot(); err != nil {
			return err
		}
		cfg = s.cfg.Get()
	}
	_, err = syncFXClient(s.client, &s.p, acc, branch, cfg.BranchDir(branch), cfg.FXGame, true, cfg.AllowMods)
	return err
}
func describeGame(root string) gameInfo {
	info := gameInfo{Type: "gameInfo", Dir: root, Clients: []gamefiles.Client{}}
	if root == "" {
		return info
	}
	info.Clients = gamefiles.InstalledClients(root)
	if info.Clients == nil {
		info.Clients = []gamefiles.Client{}
	}
	for _, dir := range gamefiles.DownloadDirs(root) {
		info.Downloads += cachefiles.DirSize(dir)
	}
	info.Free, _ = platform.DiskFree(root)
	return info
}
func describeConfiguredGame(cfg config.Config) gameInfo {
	root := cfg.Game
	if root == "" {
		root = cfg.FXGame
	}
	info := describeGame(root)
	info.BranchPaths = cfg.BranchGames
	info.Clients = installedConfiguredClients(cfg)
	if info.Clients == nil {
		info.Clients = []gamefiles.Client{}
	}
	for i := range info.Clients {
		for _, dir := range gamefiles.DownloadDirs(info.Clients[i].Dir) {
			info.Clients[i].Downloads += cachefiles.DirSize(dir)
		}
	}
	for _, other := range configuredRoots(cfg) {
		if strings.EqualFold(other, root) {
			continue
		}
		for _, cache := range gamefiles.DownloadDirs(other) {
			info.Downloads += cachefiles.DirSize(cache)
		}
	}
	return info
}
func configuredRoots(cfg config.Config) []string {
	var roots []string
	seen := map[string]bool{}
	add := func(dir string) {
		if dir == "" {
			return
		}
		key := strings.ToLower(filepath.Clean(dir))
		if !seen[key] {
			seen[key] = true
			roots = append(roots, dir)
		}
	}
	add(cfg.Game)
	add(cfg.FXGame)
	for _, client := range installedConfiguredClients(cfg) {
		add(client.Dir)
	}
	for _, dir := range cfg.BranchGames {
		add(dir)
	}
	return roots
}
func installedConfiguredClients(cfg config.Config) []gamefiles.Client {
	var clients []gamefiles.Client
	for _, client := range gamefiles.InstalledClients(cfg.Game) {
		if client.Kind == gamefiles.KindVK {
			clients = append(clients, client)
		}
	}
	for _, client := range gamefiles.InstalledClients(cfg.FXGame) {
		if client.Kind == gamefiles.KindFX || (client.Kind == gamefiles.KindBranch && strings.EqualFold(client.Dir, cfg.BranchDir(client.Branch))) {
			clients = append(clients, client)
		}
	}
	for branch, dir := range cfg.BranchGames {
		if dir == "" {
			continue
		}
		state, ok := gamefiles.ReadBranchState(dir)
		if !ok || state.Branch == gamefiles.DefaultBranch || !strings.EqualFold(state.Branch, branch) {
			continue
		}
		found := false
		for i := range clients {
			if clients[i].Kind == gamefiles.KindBranch && strings.EqualFold(clients[i].Branch, branch) {
				clients[i] = gamefiles.Client{Kind: gamefiles.KindBranch, Branch: state.Branch, Dir: dir, Version: state.Version}
				found = true
				break
			}
		}
		if !found {
			clients = append(clients, gamefiles.Client{Kind: gamefiles.KindBranch, Branch: state.Branch, Dir: dir, Version: state.Version})
		}
	}
	return clients
}
