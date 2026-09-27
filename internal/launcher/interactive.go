package launcher

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
	"github.com/TheGreatPepix/awlauncher/internal/region"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var errQuit = errors.New("quit")

type prompter struct {
	in      *bufio.Reader
	ask     func(string) string
	confirm func(string, bool) bool
	note    func(string)
}

func (p prompter) say(a ...any) {
	p.tell(strings.TrimSuffix(fmt.Sprintln(a...), "\n"))
}

func (p prompter) sayf(format string, a ...any) {
	p.tell(strings.TrimSuffix(fmt.Sprintf(format, a...), "\n"))
}

func (p prompter) tell(text string) {
	fmt.Println(text)
	if p.note != nil {
		p.note(text)
	}
}

func (p prompter) line(question string) string {
	if p.ask != nil {
		return strings.TrimSpace(p.ask(question))
	}
	fmt.Print(question)
	s, _ := p.in.ReadString('\n')
	return strings.TrimSpace(strings.TrimPrefix(s, "\ufeff"))
}

func (p prompter) yes(question string, def bool) bool {
	if p.confirm != nil {
		return p.confirm(question, def)
	}
	hint := " [y/N] "
	if def {
		hint = " [Y/n] "
	}
	switch strings.ToLower(p.line(question + hint)) {
	case "":
		return def
	case "y", "yes", "д", "да":
		return true
	}
	return false
}

func (p prompter) pause() {
	if progress.InteractiveScreen() {
		p.line("\nPress Enter to return to the menu...")
	}
}

func chooseGameDir(p prompter, cfg *configStore) (gameInstall, error) {
	if saved := cfg.get().Game; saved != "" {
		g, err := openGame(saved)
		if err == nil {
			return g, nil
		}
		if !isGameDir(saved) && p.yes(fmt.Sprintf("The game is not installed in %s (unfinished install?). Install it there?", saved), true) {
			if g, ok := installInto(p, cfg, saved); ok {
				return g, nil
			}
		} else {
			p.sayf("Saved game folder is not usable (%v).\n", err)
		}
	}
	detected := detectGameDir()
	if detected != "" && p.yes(fmt.Sprintf("Found the game in %s. Use it?", detected), true) {
		if g, err := openGame(detected); err == nil {
			return g, nil
		} else {
			p.say("Not usable:", err)
		}
	}
	if detected == "" {
		p.say("Armored Warfare was not found. Enter the folder of an existing install, or any folder to install the game into.")
	}
	for {
		dir := strings.Trim(p.line(fmt.Sprintf("Game folder [%s] (Enter for default, q to quit): ", defaultGameDir())), `"' `)
		if dir == "" {
			if p.ask != nil {
				return gameInstall{}, errQuit
			}
			dir = defaultGameDir()
		} else if strings.EqualFold(dir, "q") {
			return gameInstall{}, errQuit
		}
		g, err := openGame(dir)
		if err == nil {
			return g, nil
		}
		if isGameDir(dir) {
			p.say("Not usable:", err)
			continue
		}
		if !p.yes(fmt.Sprintf("There is no Armored Warfare install in %s. Install the game there?", dir), true) {
			continue
		}
		if g, ok := installInto(p, cfg, dir); ok {
			return g, nil
		}
	}
}

func installInto(p prompter, cfg *configStore, dir string) (gameInstall, bool) {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if state, ok := readBranchState(dir); ok && state.Branch != "" && state.Branch != fxDefaultBranch {
		p.say("Cannot install VK Play over an FX ID client.")
		return gameInstall{}, false
	}
	var previous, fxGame string
	if err := cfg.update(func(c *launcherConfig) {
		previous, c.Game = c.Game, dir
		if c.SeparateMain {
			fxGame = c.FXGame
		}
	}); err != nil {
		p.say("Error:", err)
	}
	if err := installGame(p, dir, fxGame); err != nil {
		if errors.Is(err, errInstallCancelled) {
			_ = cfg.update(func(c *launcherConfig) {
				if c.Game == dir {
					c.Game = previous
				}
			})
		} else {
			p.say("Install failed:", err)
		}
		return gameInstall{}, false
	}
	g, err := openGame(dir)
	if err != nil {
		p.say("Not usable after install:", err)
		return gameInstall{}, false
	}
	return g, true
}

func ensureUpdated(p prompter, g *gameInstall) bool {
	p.say("Checking for updates...")
	patches, latest, err := latestPatches(&http.Client{Timeout: 90 * time.Second}, g.Build)
	if err != nil {
		p.say("Update check failed:", err)
		return p.yes("Continue anyway?", false)
	}
	if len(patches) == 0 {
		cleanupCache(resolveCacheDir(g.Root), g.Build)
		p.sayf("Build %d is up to date.\n", g.Build)
		return true
	}
	p.sayf("Update available: %d -> %d (%d patches).\n", g.Build, latest, len(patches))
	if !p.yes("Install now?", true) {
		return p.yes("Start the game without updating?", false)
	}
	if err := installPatches(g.Root, patches); err != nil {
		p.say("Update failed:", err)
		return false
	}
	updated, err := openGame(g.Root)
	if err != nil {
		p.say("After update:", err)
		return false
	}
	*g = updated
	return true
}

func printAccounts(cfg *launcherConfig, def account, hasDef bool) {
	if len(cfg.Accounts) == 0 {
		fmt.Println("\nNo saved accounts. Type + to add one.")
		return
	}
	fmt.Println("\nAccounts:")
	width, service := 0, 0
	for _, a := range cfg.Accounts {
		width = max(width, len([]rune(a.label())))
		service = max(service, len(a.service()))
	}
	for i, a := range cfg.Accounts {
		mark := ""
		if hasDef && a.UserID == def.UserID {
			mark = "  <- last used"
		}
		fmt.Printf("  %d) %-*s  %-*s%s\n", i+1, width, a.label(), service, a.service(), mark)
	}
}

func printMenu(cfg *launcherConfig, def account, hasDef bool) {
	fmt.Println()
	if hasDef {
		fmt.Printf("  Enter    play as %s\n", def.label())
	}
	switch n := len(cfg.Accounts); {
	case n == 1:
		fmt.Println("  1        play as that account")
	case n > 1:
		fmt.Printf("  1..%d     play as that account\n", n)
	}
	fmt.Println("  +        add an account")
	if _, ok := fxAccountFor(cfg, def, hasDef); ok {
		fmt.Println("  b [N]    list FX ID client branches (N: account number)")
		fmt.Println("  k [N]    activate an FX ID key")
	}
	if len(cfg.Accounts) > 0 {
		fmt.Println("  r [N]    rename an account")
	}
	if cfg.Game != "" {
		fmt.Println("  g        game: check files, free space, uninstall")
	}
	if gameRunning() {
		fmt.Println("  x        close the game")
	}
	switch n := len(cfg.Accounts); {
	case n == 1:
		fmt.Println("  -1       remove that account")
	case n > 1:
		fmt.Printf("  -1..-%d   remove an account\n", n)
	}
	fmt.Println("  q        quit")
}

func fxAccountFor(cfg *launcherConfig, def account, hasDef bool) (account, bool) {
	if hasDef && def.isFX() {
		return def, true
	}
	for _, a := range cfg.Accounts {
		if a.isFX() {
			return a, true
		}
	}
	return account{}, false
}

func (s *session) pickFXAccount(arg string, def account, hasDef bool) (account, bool) {
	cfg := s.cfg.get()
	if arg != "" {
		n, err := strconv.Atoi(arg)
		if err != nil || n < 1 || n > len(cfg.Accounts) || !cfg.Accounts[n-1].isFX() {
			s.p.say("That number is not an FX ID account.")
			return account{}, false
		}
		return cfg.Accounts[n-1], true
	}
	fallback, ok := fxAccountFor(&cfg, def, hasDef)
	if !ok {
		s.p.say("Add an FX ID account first.")
		return account{}, false
	}
	var fx []int
	for i, a := range cfg.Accounts {
		if a.isFX() {
			fx = append(fx, i)
		}
	}
	if len(fx) == 1 {
		return fallback, true
	}
	s.p.say("Which FX ID account?")
	for _, i := range fx {
		s.p.sayf("  %d  %s\n", i+1, cfg.Accounts[i].label())
	}
	answer := firstWord(s.p.line("Account number (Enter for " + fallback.label() + "): "))
	if answer == "" {
		return fallback, true
	}
	return s.pickFXAccount(answer, def, hasDef)
}

func (s *session) pickAccount(arg string, def account, hasDef bool) (account, bool) {
	cfg := s.cfg.get()
	switch {
	case len(cfg.Accounts) == 0:
		return account{}, false
	case arg == "" && len(cfg.Accounts) == 1:
		return cfg.Accounts[0], true
	case arg == "":
		prompt := "Account number: "
		if hasDef {
			prompt = "Account number (Enter for " + def.label() + "): "
		}
		arg = firstWord(s.p.line(prompt))
		if arg == "" && hasDef {
			return def, true
		}
	}
	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > len(cfg.Accounts) {
		s.p.say("No such account number.")
		return account{}, false
	}
	return cfg.Accounts[n-1], true
}

func (s *session) rename(acc account) {
	name := strings.TrimSpace(s.p.line("New name for " + acc.label() + " (Enter to show the login): "))
	if err := s.cfg.updateAccount(acc.UserID, func(a *account) { a.Name = name }); err != nil {
		s.p.say("Error:", err)
	}
}

func (s *session) showBranches(acc account) {
	s.p.sayf("Asking FX ID for branches of %s...\n", acc.label())
	branches, err := fxBranches(s.client, acc)
	if errors.Is(err, errNeedLogin) {
		relogged, lerr := s.relogin(acc)
		if lerr != nil {
			s.p.say("Sign-in failed:", lerr)
			return
		}
		acc = relogged
		branches, err = fxBranches(s.client, acc)
	}
	if err != nil {
		s.p.say("Could not list branches:", err)
		return
	}
	printFXBranches(s.p, branches)
	s.chooseBranch(acc, branches)
}

func (s *session) chooseBranch(acc account, branches []fxBranchInfo) {
	var open []string
	for _, b := range branches {
		if b.Err == nil {
			open = append(open, b.Name)
		}
	}
	if len(open) == 0 || (len(open) == 1 && open[0] == fxDefaultBranch && acc.Branch == "") {
		return
	}
	current := acc.Branch
	if current == "" {
		current = fxDefaultBranch
	}
	s.p.sayf("Which branch should %s play?\n", acc.label())
	for i, name := range open {
		mark := ""
		if name == current {
			mark = "  <- current"
		}
		s.p.sayf("  %d  %s%s\n", i+1, name, mark)
	}
	n, err := strconv.Atoi(firstWord(s.p.line("Branch number (Enter to keep " + current + "): ")))
	if err != nil || n < 1 || n > len(open) {
		return
	}
	acc.Branch = open[n-1]
	if acc.Branch == fxDefaultBranch {
		acc.Branch = ""
	}
	if err := s.cfg.updateAccount(acc.UserID, func(a *account) { a.Branch = acc.Branch }); err != nil {
		s.p.say("Error:", err)
		return
	}
	if acc.Branch == "" {
		s.p.say("The account will play the main client.")
	} else {
		s.p.sayf("The account will play %s. It is installed next to the main game on the next start.\n", acc.Branch)
	}
}

func (s *session) activateKey(acc account) {
	key := strings.TrimSpace(s.p.line("Key for a closed branch (Enter to skip): "))
	if key == "" {
		return
	}
	branch, err := fxActivateKey(s.client, acc, key)
	if errors.Is(err, errNeedLogin) {
		relogged, lerr := s.relogin(acc)
		if lerr != nil {
			s.p.say("Sign-in failed:", lerr)
			return
		}
		acc = relogged
		branch, err = fxActivateKey(s.client, acc, key)
	}
	if err != nil {
		s.p.say("Key not activated:", err)
		return
	}
	s.p.sayf("Key activated: branch %q is now available to %s.\n", branch, acc.label())
	s.showBranches(acc)
}

func (s *session) addAccount() (account, error) {
	p, client, cfg := s.p, s.client, s.cfg
	p.say("Which service is the account on?")
	p.say("  1  VK Play (sign-in in the browser)")
	p.say("  2  FX ID, Wishlist Games (e-mail and a code)")
	var added account
	var err error
	switch firstWord(p.line("> ")) {
	case "1":
		added, err = loginAccount(client, cfg, "", vkSignInTimeout)
	case "2":
		added, err = loginFX(p, client, cfg, "", "")
	default:
		return account{}, errors.New("no service chosen")
	}
	if err != nil {
		return account{}, err
	}
	if name := p.line("Account name (Enter to keep " + added.label() + "): "); name != "" {
		added.Name = name
		if err := cfg.updateAccount(added.UserID, func(a *account) { a.Name = name }); err != nil {
			return account{}, err
		}
	}
	return added, nil
}

func firstWord(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return strings.ToLower(f[0])
	}
	return ""
}

type session struct {
	p      prompter
	cfg    *configStore
	client *http.Client
	found  *foundGame
}

type foundGame struct{ game *gameInstall }

func (s *session) with(p prompter) *session {
	c := *s
	c.p = p
	return &c
}

func (s *session) vkGame() (gameInstall, error) {
	if s.found.game == nil {
		g, err := chooseGameDir(s.p, s.cfg)
		if err != nil {
			return gameInstall{}, err
		}
		if s.cfg.get().Game != g.Root {
			if err := s.cfg.update(func(c *launcherConfig) { c.Game = g.Root }); err != nil {
				return gameInstall{}, err
			}
		}
		s.p.sayf("Game: %s (build %d)\n", g.Root, g.Build)
		if !ensureUpdated(s.p, &g) {
			return gameInstall{}, errQuit
		}
		s.found.game = &g
	}
	return *s.found.game, nil
}

func (s *session) sharedRoot() (string, error) {
	if saved := s.cfg.get().Game; saved != "" {
		return saved, nil
	}
	s.p.say("Choose a folder for the main client shared by FX ID and VK Play. Press Enter for the default, or q to return to the menu.")
	for {
		dir := strings.Trim(s.p.line(fmt.Sprintf("Game folder [%s]: ", defaultGameDir())), `"' `)
		if dir == "" {
			if s.p.ask != nil {
				return "", errQuit
			}
			dir = defaultGameDir()
		} else if strings.EqualFold(dir, "q") {
			return "", errQuit
		}
		root, err := filepath.Abs(dir)
		if err != nil {
			s.p.say("Invalid folder:", err)
			continue
		}
		if state, ok := readBranchState(root); ok && state.Branch != fxDefaultBranch {
			s.p.sayf("That folder contains the FX ID %s branch. Choose another folder.\n", state.Branch)
			continue
		}
		if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 {
			_, fxInstall := readBranchState(root)
			if !fxInstall && !isGameDir(root) && !s.p.yes("The folder is not empty. Continue there?", false) {
				continue
			}
		}
		if err := s.cfg.update(func(c *launcherConfig) { c.Game = root }); err != nil {
			return "", err
		}
		return root, nil
	}
}

func (s *session) fxRoot() (string, error) {
	cfg := s.cfg.get()
	if !cfg.SeparateMain {
		return s.sharedRoot()
	}
	if cfg.FXGame != "" {
		return cfg.FXGame, nil
	}
	defaultDir := defaultGameDir() + " FX ID"
	for {
		dir := strings.Trim(s.p.line(fmt.Sprintf("FX ID game folder [%s]: ", defaultDir)), `"' `)
		if dir == "" {
			if s.p.ask != nil {
				return "", errQuit
			}
			dir = defaultDir
		} else if strings.EqualFold(dir, "q") {
			return "", errQuit
		}
		root, err := filepath.Abs(dir)
		if err != nil {
			s.p.say("Invalid folder:", err)
			continue
		}
		if strings.EqualFold(root, cfg.Game) || isGameDir(root) {
			s.p.say("Choose a folder different from the VK Play client.")
			continue
		}
		if state, ok := readBranchState(root); ok && state.Branch != fxDefaultBranch {
			s.p.say("That folder contains a closed FX ID branch.")
			continue
		}
		if err := s.cfg.update(func(c *launcherConfig) { c.FXGame = root }); err != nil {
			return "", err
		}
		return root, nil
	}
}

func (s *session) play(acc account) error {
	switch {
	case acc.isFX() && acc.Branch != "":
		cfg := s.cfg.get()
		root := cfg.Game
		if cfg.SeparateMain {
			root = cfg.FXGame
		}
		if root == "" && cfg.BranchGames[strings.ToLower(acc.Branch)] == "" {
			var err error
			root, err = s.fxRoot()
			if err != nil {
				return err
			}
		}
		return playFXInstall(s.client, &s.p, acc, acc.Branch, s.cfg.get().branchDir(acc.Branch), root, false, s.cfg.get().AllowMods)
	case acc.isFX():
		root, err := s.fxRoot()
		if err != nil {
			return err
		}
		return playFXInstall(s.client, &s.p, acc, fxDefaultBranch, root, "", true, s.cfg.get().AllowMods)
	}
	g, err := s.vkGame()
	if err != nil {
		return err
	}
	if err := region.Ensure(g.Root, region.VK); err != nil {
		return err
	}
	if err := verifyClientBeforeLaunch(s.p, g, s.cfg.get().AllowMods); err != nil {
		return err
	}
	return startGame(s.client, g, acc)
}

func (s *session) relogin(acc account) (account, error) {
	s.p.sayf("Session for %s has expired, sign in again.\n", acc.label())
	var relogged account
	var err error
	if acc.isFX() {
		relogged, err = loginFX(s.p, s.client, s.cfg, acc.Email, acc.Name)
	} else {
		relogged, err = loginAccount(s.client, s.cfg, acc.Name, vkSignInTimeout)
	}
	if err != nil {
		return account{}, err
	}
	if relogged.UserID != acc.UserID {
		s.p.sayf("Signed in to a different account (%s); it was added to the list.\n", relogged.label())
	}
	return relogged, nil
}

func runInteractive() error {
	loaded, err := loadConfig()
	if err != nil {
		return err
	}
	store := newConfigStore(loaded)
	p := prompter{in: bufio.NewReader(os.Stdin)}
	s := &session{p: p, cfg: store, client: authClient(), found: &foundGame{}}
	var tray *trayController
	defer func() {
		if tray != nil {
			tray.close()
		}
	}()
	for {
		progress.ClearScreen()
		fmt.Println("AWLauncher")
		cfg := store.get()
		def, hasDef := cfg.defaultAccount()
		printAccounts(&cfg, def, hasDef)
		printMenu(&cfg, def, hasDef)
		words := strings.Fields(strings.ToLower(p.line("> ")))
		answer, arg := "", ""
		if len(words) > 0 {
			answer = words[0]
		}
		if len(words) > 1 {
			arg = words[1]
		}
		var acc account
		switch {
		case answer == "q" || answer == "й":
			return nil
		case answer == "k" || answer == "л":
			progress.ClearScreen()
			if fx, ok := s.pickFXAccount(arg, def, hasDef); ok {
				s.activateKey(fx)
			}
			p.pause()
			continue
		case answer == "r" || answer == "к":
			progress.ClearScreen()
			if acc, ok := s.pickAccount(arg, def, hasDef); ok {
				s.rename(acc)
			}
			continue
		case answer == "g" || answer == "п":
			progress.ClearScreen()
			s.gameMenu()
			p.pause()
			continue
		case answer == "x" || answer == "ч":
			if !p.yes("Close Armored Warfare? If you are in a battle, you leave it.", false) {
				continue
			}
			fmt.Println("Closing the game...")
			if err := closeGame(10 * time.Second); err != nil {
				fmt.Println("Error:", err)
				p.pause()
			}
			continue
		case answer == "b" || answer == "и":
			progress.ClearScreen()
			if fx, ok := s.pickFXAccount(arg, def, hasDef); ok {
				s.showBranches(fx)
			}
			p.pause()
			continue
		case answer == "" && hasDef:
			acc = def
		case answer == "+", answer == "" && len(cfg.Accounts) == 0:
			progress.ClearScreen()
			added, err := s.addAccount()
			if err != nil {
				fmt.Println("Sign-in failed:", err)
				p.pause()
				continue
			}
			if !p.yes("Start the game with this account?", true) {
				continue
			}
			acc, _ = store.find(added.UserID)
		case strings.HasPrefix(answer, "-"):
			n, err := strconv.Atoi(answer[1:])
			if err != nil || n < 1 || n > len(cfg.Accounts) {
				fmt.Println("No such account number.")
				continue
			}
			victim := cfg.Accounts[n-1]
			if !p.yes("Remove account "+victim.label()+"?", false) {
				continue
			}
			if victim.isFX() {
				err = clearRefreshToken(victim.UserID)
			} else {
				err = dropSession(victim.UserID)
			}
			if err != nil {
				fmt.Println("Error:", err)
			}
			if err := store.update(func(c *launcherConfig) { c.remove(victim.UserID) }); err != nil {
				return err
			}
			continue
		default:
			n, err := strconv.Atoi(answer)
			if err != nil || n < 1 || n > len(cfg.Accounts) {
				fmt.Println("Type an account number, +, -number or q.")
				continue
			}
			acc = cfg.Accounts[n-1]
		}
		progress.ClearScreen()
		fmt.Printf("AWLauncher - %s\n\n", acc.label())
		err := s.play(acc)
		if errors.Is(err, errNeedLogin) {
			relogged, lerr := s.relogin(acc)
			if lerr != nil {
				fmt.Println("Sign-in failed:", lerr)
				p.pause()
				continue
			}
			acc = relogged
			err = s.play(acc)
		}
		if errors.Is(err, errQuit) {
			continue
		}
		if err != nil {
			fmt.Println("Game not started:", err)
			p.pause()
			continue
		}
		if err := store.update(func(c *launcherConfig) { c.LastUserID = acc.UserID }); err != nil {
			return err
		}
		if ownsConsole() {
			if tray == nil {
				tray, err = newTrayController()
				if err != nil {
					fmt.Println("Could not open the tray:", err)
					p.pause()
					continue
				}
			}
			fmt.Println("Launcher is hidden in the notification area. Use its icon to reopen or exit.")
			if !tray.hideUntilRestored() {
				return nil
			}
			continue
		}
		fmt.Println("The launcher remains open while the game runs.")
		if !progress.InteractiveScreen() {
			return nil
		}
		p.pause()
	}
}
