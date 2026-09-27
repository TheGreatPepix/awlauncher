package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/fileutil"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
	"github.com/TheGreatPepix/awlauncher/internal/region"
)

const (
	clientVK     = "vkplay"
	clientFX     = "fxid"
	clientBranch = "branch"
)

type gameClient struct {
	Kind    string `json:"kind"`
	Branch  string `json:"branch,omitempty"`
	Dir     string `json:"dir"`
	Version string `json:"version"`
}

func (c gameClient) name() string {
	switch c.Kind {
	case clientVK:
		return "VK Play"
	case clientFX:
		return "FX ID main branch"
	}
	return "FX ID " + c.Branch
}

type gameInfo struct {
	Type        string            `json:"type"`
	Dir         string            `json:"dir"`
	Clients     []gameClient      `json:"clients"`
	Downloads   int64             `json:"downloads"`
	Free        int64             `json:"free"`
	Region      string            `json:"region,omitempty"`
	BranchPaths map[string]string `json:"branchPaths,omitempty"`
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
	for _, acc := range s.cfg.get().Accounts {
		kind := clientVK
		if acc.isFX() {
			kind = clientFX
		} else {
			if !seen[kind] {
				client := availableClient{Kind: kind, Account: strconv.FormatInt(acc.UserID, 10)}
				if distrib, err := latestDistrib(s.client); err == nil {
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
			if !branchNamePattern.MatchString(b.Name) {
				continue
			}
			k := strings.ToLower(b.Name)
			if seen[k] {
				continue
			}
			clientKind := clientBranch
			if strings.EqualFold(b.Name, fxDefaultBranch) {
				clientKind = clientFX
			}
			out.Clients = append(out.Clients, availableClient{Kind: clientKind, Branch: b.Name, Account: strconv.FormatInt(acc.UserID, 10), Version: b.Version})
			seen[k] = true
		}
	}
	return out
}

func (s *session) downloadClient(acc account, kind, branch string) error {
	if err := ensureGameClosed(); err != nil {
		return err
	}
	if kind == clientVK {
		if acc.isFX() {
			return errors.New("a VK Play account is required")
		}
		root, err := s.sharedRoot()
		if err != nil {
			return err
		}
		fxRoot := ""
		if cfg := s.cfg.get(); cfg.SeparateMain {
			fxRoot = cfg.FXGame
		}
		return installGame(s.p, root, fxRoot)
	}
	if !acc.isFX() || !branchNamePattern.MatchString(branch) {
		return errors.New("an FX ID account and valid branch are required")
	}
	if (kind == clientFX) != strings.EqualFold(branch, fxDefaultBranch) || (kind != clientFX && kind != clientBranch) {
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
	cfg := s.cfg.get()
	root := cfg.Game
	if kind == clientFX {
		var err error
		root, err = s.fxRoot()
		if err != nil {
			return err
		}
	} else if kind == clientBranch && cfg.SeparateMain {
		root = cfg.FXGame
		if root == "" && cfg.BranchGames[strings.ToLower(branch)] == "" {
			var err error
			root, err = s.fxRoot()
			if err != nil {
				return err
			}
		}
	} else if root == "" && cfg.BranchGames[strings.ToLower(branch)] == "" {
		var err error
		root, err = s.sharedRoot()
		if err != nil {
			return err
		}
	}
	clientRoot, mainRoot := root, ""
	if kind == clientBranch {
		cfg := s.cfg.get()
		if cfg.SeparateMain {
			mainRoot = cfg.FXGame
		} else {
			mainRoot = root
		}
		clientRoot = cfg.branchDir(branch)
	}
	_, err = syncFXClient(s.client, &s.p, acc, branch, clientRoot, mainRoot, true, s.cfg.get().AllowMods)
	return err
}

func describeGame(root string) gameInfo {
	info := gameInfo{Type: "gameInfo", Dir: root, Clients: []gameClient{}}
	if root == "" {
		return info
	}
	info.Clients = installedClients(root)
	if info.Clients == nil {
		info.Clients = []gameClient{}
	}
	for _, dir := range downloadDirs(root) {
		info.Downloads += dirSize(dir)
	}
	info.Free, _ = diskFree(root)
	if len(info.Clients) > 0 {
		info.Region = region.Current(root)
	}
	return info
}

func describeConfiguredGame(cfg launcherConfig) gameInfo {
	info := describeGame(cfg.Game)
	info.BranchPaths = cfg.BranchGames
	info.Clients = installedConfiguredClients(cfg)
	if info.Clients == nil {
		info.Clients = []gameClient{}
	}
	for _, root := range configuredRoots(cfg) {
		if strings.EqualFold(root, cfg.Game) {
			continue
		}
		for _, cache := range downloadDirs(root) {
			info.Downloads += dirSize(cache)
		}
	}
	return info
}

func configuredRoots(cfg launcherConfig) []string {
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
	if cfg.SeparateMain {
		add(cfg.FXGame)
	}
	for _, client := range installedConfiguredClients(cfg) {
		add(client.Dir)
	}
	for _, dir := range cfg.BranchGames {
		add(dir)
	}
	return roots
}

func (c launcherConfig) branchDir(branch string) string {
	if dir := c.BranchGames[strings.ToLower(branch)]; dir != "" {
		return dir
	}
	root := c.Game
	if c.SeparateMain {
		root = c.FXGame
	}
	if root == "" {
		root = defaultGameDir()
		if c.SeparateMain {
			root += " FX ID"
		}
	}
	return branchInstallDir(root, branch)
}

func installedConfiguredClients(cfg launcherConfig) []gameClient {
	clients := installedClients(cfg.Game)
	if cfg.SeparateMain && cfg.FXGame != "" {
		clients = append(clients, branchClients(cfg.FXGame)...)
	}
	filtered := clients[:0]
	for _, client := range clients {
		if cfg.SeparateMain && client.Kind == clientFX {
			continue
		}
		if client.Kind != clientBranch || strings.EqualFold(client.Dir, cfg.branchDir(client.Branch)) {
			filtered = append(filtered, client)
		}
	}
	clients = filtered
	if cfg.SeparateMain && cfg.FXGame != "" {
		if state, ok := readBranchState(cfg.FXGame); ok && state.Branch == fxDefaultBranch {
			clients = append(clients, gameClient{Kind: clientFX, Branch: fxDefaultBranch, Dir: cfg.FXGame, Version: state.Version})
		}
	}
	for branch, dir := range cfg.BranchGames {
		if dir == "" {
			continue
		}
		state, ok := readBranchState(dir)
		if !ok || state.Branch == fxDefaultBranch || !strings.EqualFold(state.Branch, branch) {
			continue
		}
		found := false
		for i := range clients {
			if clients[i].Kind == clientBranch && strings.EqualFold(clients[i].Branch, branch) {
				clients[i] = gameClient{Kind: clientBranch, Branch: state.Branch, Dir: dir, Version: state.Version}
				found = true
				break
			}
		}
		if !found {
			clients = append(clients, gameClient{Kind: clientBranch, Branch: state.Branch, Dir: dir, Version: state.Version})
		}
	}
	return clients
}

func installedClients(root string) []gameClient {
	var clients []gameClient
	if root == "" {
		return clients
	}
	if isGameDir(root) {
		version := "installed"
		if build, _, err := currentBuild(root); err == nil {
			version = "build " + strconv.Itoa(build)
		}
		clients = append(clients, gameClient{Kind: clientVK, Dir: root, Version: version})
	}
	if s, ok := readBranchState(root); ok && s.Branch == fxDefaultBranch {
		clients = append(clients, gameClient{Kind: clientFX, Branch: fxDefaultBranch, Dir: root, Version: s.Version})
	}
	return append(clients, branchClients(root)...)
}

func branchClients(root string) []gameClient {
	parent, base := filepath.Dir(root), filepath.Base(root)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	var clients []gameClient
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(strings.ToLower(e.Name()), strings.ToLower(base)+" ") {
			continue
		}
		dir := filepath.Join(parent, e.Name())
		s, ok := readBranchState(dir)
		if !ok || s.Branch == fxDefaultBranch || !strings.EqualFold(branchInstallDir(root, s.Branch), dir) {
			continue
		}
		clients = append(clients, gameClient{Kind: clientBranch, Branch: s.Branch, Dir: dir, Version: s.Version})
	}
	return clients
}

func downloadDirs(root string) []string {
	return []string{
		filepath.Join(root, "-gup-", "awlauncher-cache"),
		filepath.Join(root, "-gup-", "awpatcher-cache"),
	}
}

func (s *session) gameRoot() (string, error) {
	root := s.cfg.get().Game
	if root == "" {
		return "", errors.New("no game folder is set yet")
	}
	return root, nil
}

func (s *session) findClient(kind, branch string) (gameClient, error) {
	root := s.cfg.get().Game
	for _, c := range installedConfiguredClients(s.cfg.get()) {
		if c.Kind == kind && (kind != clientBranch || strings.EqualFold(c.Branch, branch)) {
			return c, nil
		}
	}
	return gameClient{}, fmt.Errorf("that client is not installed in %s", root)
}

func (s *session) chooseClient(question string) (gameClient, error) {
	root := s.cfg.get().Game
	clients := installedConfiguredClients(s.cfg.get())
	switch len(clients) {
	case 0:
		return gameClient{}, fmt.Errorf("the game is not installed in %s", root)
	case 1:
		return clients[0], nil
	}
	for i, c := range clients {
		s.p.sayf("  %d  %s, %s\n", i+1, c.name(), c.Version)
	}
	n, err := strconv.Atoi(firstWord(s.p.line(question)))
	if err != nil || n < 1 || n > len(clients) {
		return gameClient{}, errQuit
	}
	return clients[n-1], nil
}

func (s *session) verifyClient(c gameClient) error {
	if err := ensureGameClosed(); err != nil {
		return err
	}
	if c.Kind == clientVK {
		return s.verifyVK(c.Dir)
	}
	acc, ok := s.fxAccountFor(c.Branch)
	if !ok {
		return errors.New("add an FX ID account first: the file list comes from FX ID")
	}
	if err := markBranchDirty(c.Dir); err != nil {
		return err
	}
	root := s.cfg.get().Game
	if cfg := s.cfg.get(); cfg.SeparateMain {
		root = cfg.FXGame
	}
	state, err := syncFXClient(s.client, &s.p, acc, c.Branch, c.Dir, root, false, false)
	if errors.Is(err, errNeedLogin) {
		if acc, err = s.relogin(acc); err != nil {
			return err
		}
		state, err = syncFXClient(s.client, &s.p, acc, c.Branch, c.Dir, root, false, false)
	}
	if err != nil {
		return err
	}
	s.p.sayf("%s %s: all %d files are checked.\n", c.name(), state.Version, len(state.Files))
	return nil
}

func (s *session) verifyVK(root string) error {
	g, err := openGame(root)
	if err != nil {
		return err
	}
	patches, latest, err := latestPatches(&http.Client{Timeout: 90 * time.Second}, g.Build)
	if err != nil {
		return fmt.Errorf("update check: %w", err)
	}
	if len(patches) > 0 {
		s.p.sayf("Update available: %d -> %d. Files are checked against the latest build.\n", g.Build, latest)
		if !s.p.yes("Install the update first?", true) {
			return errQuit
		}
		if err := installPatches(root, patches); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
		if g, err = openGame(root); err != nil {
			return err
		}
	}
	s.found.game = &g
	if err := region.Ensure(root, region.VK); err != nil {
		return err
	}
	if err := fileutil.WriteAtomic(vkVerificationMarker(root), []byte("1")); err != nil {
		return err
	}
	if err := verifyClientBeforeLaunch(s.p, g, false); err != nil {
		return err
	}
	s.p.sayf("VK Play build %d: every file matches the official list.\n", g.Build)
	return nil
}

func (s *session) fxAccountFor(branch string) (account, bool) {
	want := branch
	if want == fxDefaultBranch {
		want = ""
	}
	cfg := s.cfg.get()
	if a, ok := cfg.find(cfg.LastUserID); ok && a.isFX() && strings.EqualFold(a.Branch, want) {
		return a, true
	}
	var fallback account
	found := false
	for _, a := range cfg.Accounts {
		if !a.isFX() {
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

func markBranchDirty(root string) error {
	state, ok := readBranchState(root)
	if !ok {
		return nil
	}
	state.Dirty = true
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return fileutil.WriteAtomic(branchStatePath(root), data)
}

func (s *session) clearDownloads() error {
	var size int64
	var dirs []string
	for _, root := range configuredRoots(s.cfg.get()) {
		for _, dir := range downloadDirs(root) {
			if n := dirSize(dir); n > 0 {
				size += n
				dirs = append(dirs, dir)
			}
		}
	}
	if size == 0 {
		s.p.say("There are no downloaded patch files.")
		return nil
	}
	if !s.p.yes(fmt.Sprintf("Delete %s of downloaded patches and backups of replaced files?", progress.FormatBytes(size)), true) {
		return errQuit
	}
	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	s.p.sayf("Freed %s.\n", progress.FormatBytes(size))
	return nil
}

func (s *session) removeBranch(c gameClient) error {
	if c.Kind != clientBranch {
		return errors.New("only a closed branch can be removed on its own")
	}
	if err := ensureGameClosed(); err != nil {
		return err
	}
	if !s.p.yes(fmt.Sprintf("Remove FX ID %s %s from %s?", c.Branch, c.Version, c.Dir), false) {
		return errQuit
	}
	if err := s.removeClientDir(c.Dir); err != nil {
		return err
	}
	s.p.sayf("FX ID %s is removed.\n", c.Branch)
	return nil
}

func (s *session) uninstallGame() error {
	if err := ensureGameClosed(); err != nil {
		return err
	}
	clients := installedConfiguredClients(s.cfg.get())
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
		total += dirSize(c.Dir)
		s.p.sayf("  %s, %s: %s\n", c.name(), c.Version, c.Dir)
	}
	if !s.p.yes(fmt.Sprintf("Uninstall Armored Warfare and free about %s? Your accounts stay in the launcher.", progress.FormatBytes(total)), false) {
		return errQuit
	}
	for _, dir := range dirs {
		if err := s.removeClientDir(dir); err != nil {
			return err
		}
	}
	s.found.game = nil
	if err := s.cfg.update(func(c *launcherConfig) {
		c.Game = ""
		c.FXGame = ""
		c.BranchGames = nil
	}); err != nil {
		return err
	}
	s.p.say("Armored Warfare is uninstalled.")
	return nil
}

func (s *session) removeClientDir(dir string) error {
	names := []string{"user.cfg"}
	if isGameDir(dir) {
		if build, _, err := currentBuild(dir); err == nil {
			if inv, err := loadClientInventory(dir, build); err == nil {
				for _, f := range inv.Files {
					names = append(names, f.Name)
				}
			}
		}
	}
	if st, ok := readBranchState(dir); ok {
		for _, f := range st.Files {
			names = append(names, f.Path)
		}
	}
	for _, name := range names {
		path, err := safeGamePath(dir, name)
		if err != nil {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.RemoveAll(filepath.Join(dir, "-gup-")); err != nil {
		return err
	}
	removeEmptyDirs(dir)
	left, size := countFiles(dir)
	if left == 0 {
		return nil
	}
	s.p.sayf("%d other files (%s) are left in %s: game settings, logs, screenshots and anything else put there.\n", left, progress.FormatBytes(size), dir)
	if !removableFolder(dir) {
		return nil
	}
	if s.p.yes("Delete the folder with everything in it?", false) {
		return os.RemoveAll(dir)
	}
	return nil
}

func removeEmptyDirs(root string) {
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		_ = os.Remove(dir)
	}
}

func countFiles(root string) (int, int64) {
	var n int
	var size int64
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
			if info, err := d.Info(); err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return n, size
}

func removableFolder(dir string) bool {
	dir = filepath.Clean(dir)
	if filepath.Dir(dir) == dir || !filepath.IsAbs(dir) {
		return false
	}
	for _, protected := range protectedDirs() {
		if protected == "" {
			continue
		}
		protected = filepath.Clean(protected)
		if strings.EqualFold(dir, protected) || strings.HasPrefix(strings.ToLower(protected), strings.ToLower(dir)+string(filepath.Separator)) {
			return false
		}
	}
	if home := homeDir(); home != "" && strings.EqualFold(filepath.Dir(dir), filepath.Clean(home)) {
		return false
	}
	return true
}

func (s *session) gameMenu() {
	root := s.cfg.get().Game
	if root == "" {
		s.p.say("No game folder is set yet. It is chosen on the first start.")
	}
	info := describeConfiguredGame(s.cfg.get())
	if root != "" {
		s.p.sayf("Game folder: %s\n", root)
	}
	if len(info.Clients) == 0 {
		s.p.say("  The game is not installed there.")
	}
	branches := 0
	for _, c := range info.Clients {
		if c.Kind == clientBranch {
			branches++
			s.p.sayf("  %s, %s: %s\n", c.name(), c.Version, c.Dir)
		} else {
			s.p.sayf("  %s, %s\n", c.name(), c.Version)
		}
	}
	s.p.sayf("Downloaded patches: %s. Free space: %s.\n", progress.FormatBytes(info.Downloads), progress.FormatBytes(info.Free))
	s.p.say("")
	s.p.say("  v        check and repair the game files")
	s.p.say("  m        set the VK Play/main game folder")
	if s.cfg.get().SeparateMain {
		s.p.say("  f        set the FX ID main folder")
	}
	s.p.say("  p        set a closed branch folder")
	s.p.say("  t        choose shared or separate main clients")
	if s.cfg.get().SeparateMain {
		s.p.sayf("FX ID folder: %s\n", s.cfg.get().FXGame)
	}
	s.p.say("  c        delete downloaded patches")
	if branches > 0 {
		s.p.say("  r        remove a closed branch")
	}
	s.p.say("  u        uninstall the game")
	if root != "" {
		s.p.say("  o        open the game folder")
	}
	s.p.say("  Enter    back")
	var err error
	switch firstWord(s.p.line("> ")) {
	case "v", "м":
		var c gameClient
		if c, err = s.chooseClient("Client number: "); err == nil {
			err = s.verifyClient(c)
		}
	case "m":
		err = s.setMainFolder(false)
	case "f":
		if s.cfg.get().SeparateMain {
			err = s.setMainFolder(true)
		}
	case "p", "з":
		err = s.setBranchFolder()
	case "t", "е":
		answer := strings.ToLower(strings.TrimSpace(s.p.line("Main clients: shared or separate [s/d]: ")))
		if answer == "s" || answer == "d" {
			err = s.cfg.update(func(c *launcherConfig) { c.setSeparateMain(answer == "d") })
		}
	case "c", "с":
		err = s.clearDownloads()
	case "r", "к":
		var c gameClient
		if c, err = s.chooseInstalledBranch(); err == nil {
			err = s.removeBranch(c)
		}
	case "u", "г":
		err = s.uninstallGame()
	case "o", "щ":
		if root != "" {
			openInShell(root)
		}
	}
	if err != nil && !errors.Is(err, errQuit) {
		s.p.say("Error:", err)
	}
}

func (s *session) setMainFolder(fx bool) error {
	if err := ensureGameClosed(); err != nil {
		return err
	}
	cfg := s.cfg.get()
	label, current := "VK Play", cfg.Game
	if fx {
		label, current = "FX ID", cfg.FXGame
	}
	if current == "" {
		current = defaultGameDir()
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
		if strings.EqualFold(dir, cfg.Game) || isGameDir(dir) {
			return errors.New("choose a folder different from the VK Play client")
		}
		if state, ok := readBranchState(dir); ok && state.Branch != fxDefaultBranch {
			return errors.New("that folder contains a closed FX ID branch")
		}
	} else {
		if cfg.SeparateMain && strings.EqualFold(dir, cfg.FXGame) {
			return errors.New("choose a folder different from the FX ID client")
		}
		if state, ok := readBranchState(dir); ok && state.Branch != fxDefaultBranch {
			return errors.New("that folder contains a closed FX ID branch")
		}
	}
	for _, branchDir := range cfg.BranchGames {
		if strings.EqualFold(dir, branchDir) {
			return errors.New("that folder is assigned to a closed branch")
		}
	}
	if err := s.cfg.update(func(c *launcherConfig) {
		if fx {
			c.FXGame = dir
		} else {
			c.Game = dir
		}
	}); err != nil {
		return err
	}
	s.found.game = nil
	s.p.sayf("%s game folder: %s\n", label, dir)
	return nil
}

func (s *session) setBranchFolder() error {
	branch := strings.TrimSpace(s.p.line("FX ID branch name: "))
	if !branchNamePattern.MatchString(branch) || strings.EqualFold(branch, fxDefaultBranch) {
		return errors.New("enter a valid closed branch name")
	}
	current := s.cfg.get().branchDir(branch)
	dir := strings.Trim(s.p.line(fmt.Sprintf("Folder [%s]: ", current)), `"' `)
	if dir == "" {
		dir = current
	}
	var err error
	dir, err = filepath.Abs(dir)
	if err != nil {
		return err
	}
	cfg := s.cfg.get()
	if strings.EqualFold(dir, cfg.Game) || isGameDir(dir) {
		return errors.New("choose a folder different from the main game client")
	}
	if cfg.SeparateMain && strings.EqualFold(dir, cfg.FXGame) {
		return errors.New("choose a folder different from the FX ID main client")
	}
	for other, saved := range cfg.BranchGames {
		if !strings.EqualFold(other, branch) && strings.EqualFold(dir, saved) {
			return errors.New("that folder is assigned to another branch")
		}
	}
	if state, ok := readBranchState(dir); ok && !strings.EqualFold(state.Branch, branch) {
		return errors.New("that folder contains a different FX ID branch")
	}
	if err := s.cfg.update(func(c *launcherConfig) {
		if c.BranchGames == nil {
			c.BranchGames = map[string]string{}
		}
		c.BranchGames[strings.ToLower(branch)] = dir
	}); err != nil {
		return err
	}
	s.p.sayf("FX ID %s folder: %s\n", branch, dir)
	return nil
}

func (s *session) chooseInstalledBranch() (gameClient, error) {
	var branches []gameClient
	for _, c := range installedConfiguredClients(s.cfg.get()) {
		if c.Kind == clientBranch {
			branches = append(branches, c)
		}
	}
	switch len(branches) {
	case 0:
		return gameClient{}, errors.New("no closed branches are installed")
	case 1:
		return branches[0], nil
	}
	for i, c := range branches {
		s.p.sayf("  %d  %s, %s\n", i+1, c.name(), c.Version)
	}
	n, err := strconv.Atoi(firstWord(s.p.line("Branch number: ")))
	if err != nil || n < 1 || n > len(branches) {
		return gameClient{}, errQuit
	}
	return branches[n-1], nil
}
