package gamefiles

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	KindVK     = "vkplay"
	KindFX     = "fxid"
	KindBranch = "branch"
)

type Client struct {
	Kind      string `json:"kind"`
	Branch    string `json:"branch,omitempty"`
	Dir       string `json:"dir"`
	Version   string `json:"version"`
	Downloads int64  `json:"downloads,omitempty"`
}

func (c Client) Name() string {
	switch c.Kind {
	case KindVK:
		return "VK Play"
	case KindFX:
		return "FX ID main branch"
	}
	return "FX ID " + c.Branch
}

func InstalledClients(root string) []Client {
	var clients []Client
	if root == "" {
		return clients
	}
	if IsVKInstall(root) {
		version := "installed"
		if build, _, err := CurrentBuild(root); err == nil {
			version = "build " + strconv.Itoa(build)
		}
		clients = append(clients, Client{Kind: KindVK, Dir: root, Version: version})
	}
	if s, ok := ReadBranchState(root); ok && s.Branch == DefaultBranch {
		clients = append(clients, Client{Kind: KindFX, Branch: DefaultBranch, Dir: root, Version: s.Version})
	}
	return append(clients, BranchClients(root)...)
}

func BranchClients(root string) []Client {
	parent, base := filepath.Dir(root), filepath.Base(root)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	var clients []Client
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(strings.ToLower(e.Name()), strings.ToLower(base)+" ") {
			continue
		}
		dir := filepath.Join(parent, e.Name())
		s, ok := ReadBranchState(dir)
		if !ok || s.Branch == DefaultBranch || !strings.EqualFold(BranchDir(root, s.Branch), dir) {
			continue
		}
		clients = append(clients, Client{Kind: KindBranch, Branch: s.Branch, Dir: dir, Version: s.Version})
	}
	return clients
}

func DownloadDirs(root string) []string {
	return []string{
		filepath.Join(root, "-gup-", "awlauncher-cache"),
		filepath.Join(root, "-gup-", "awpatcher-cache"),
	}
}
