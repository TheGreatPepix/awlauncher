package launcher

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/cache"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/catalog"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

var errQuit = gamefiles.ErrDeclined

type prompter struct {
	in      *bufio.Reader
	ask     func(string) string
	confirm func(string, bool) bool
	note    func(string)
	notify  func(string)
}

func (p prompter) Say(a ...any) {
	p.tell(strings.TrimSuffix(fmt.Sprintln(a...), "\n"))
}

func (p prompter) Sayf(format string, a ...any) {
	p.tell(strings.TrimSuffix(fmt.Sprintf(format, a...), "\n"))
}

func (p prompter) Notify(text string) {
	p.tell(text + ".")
	if p.notify != nil {
		p.notify(text)
	}
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

func (p prompter) Yes(question string, def bool) bool {
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

func chooseGameDir(p prompter, cfg *config.Store) (gamefiles.Install, error) {
	if saved := cfg.Get().Game; saved != "" {
		g, err := gamefiles.Open(saved)
		if err == nil {
			return g, nil
		}
		if !gamefiles.IsVKInstall(saved) && p.Yes(fmt.Sprintf("The game is not installed in %s (unfinished install?). Install it there?", saved), true) {
			if g, ok := installInto(p, cfg, saved); ok {
				return g, nil
			}
		} else {
			p.Sayf("Saved game folder is not usable (%v).\n", err)
		}
	}
	detected := gamefiles.DetectVKInstall()
	if detected != "" && p.Yes(fmt.Sprintf("Found the game in %s. Use it?", detected), true) {
		if g, err := gamefiles.Open(detected); err == nil {
			return g, nil
		} else {
			p.Say("Not usable:", err)
		}
	}
	if detected == "" {
		p.Say("Armored Warfare was not found. Enter the folder of an existing install, or any folder to install the game into.")
	}
	for {
		dir := strings.Trim(p.line(fmt.Sprintf("Game folder [%s] (Enter for default, q to quit): ", platform.DefaultGameDir())), `"' `)
		if dir == "" {
			if p.ask != nil {
				return gamefiles.Install{}, errQuit
			}
			dir = platform.DefaultGameDir()
		} else if strings.EqualFold(dir, "q") {
			return gamefiles.Install{}, errQuit
		}
		g, err := gamefiles.Open(dir)
		if err == nil {
			return g, nil
		}
		if gamefiles.IsVKInstall(dir) {
			p.Say("Not usable:", err)
			continue
		}
		if !p.Yes(fmt.Sprintf("There is no Armored Warfare install in %s. Install the game there?", dir), true) {
			continue
		}
		if g, ok := installInto(p, cfg, dir); ok {
			return g, nil
		}
	}
}

func installInto(p prompter, cfg *config.Store, dir string) (gamefiles.Install, bool) {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if _, ok := gamefiles.ReadBranchState(dir); ok || strings.EqualFold(dir, cfg.Get().FXGame) {
		p.Say("Cannot install VK Play over an FX ID client.")
		return gamefiles.Install{}, false
	}
	var previous string
	if err := cfg.Update(func(c *config.Config) { previous, c.Game = c.Game, dir }); err != nil {
		p.Say("Error:", err)
	}
	if err := gamefiles.InstallVK(p, dir); err != nil {
		if errors.Is(err, gamefiles.ErrInstallCancelled) {
			_ = cfg.Update(func(c *config.Config) {
				if c.Game == dir {
					c.Game = previous
				}
			})
		} else {
			p.Say("Install failed:", err)
		}
		return gamefiles.Install{}, false
	}
	g, err := gamefiles.Open(dir)
	if err != nil {
		p.Say("Not usable after install:", err)
		return gamefiles.Install{}, false
	}
	p.Notify(fmt.Sprintf("VK Play build %d is installed", g.Build))
	return g, true
}

func ensureUpdated(p prompter, g *gamefiles.Install) bool {
	p.Say("Checking for updates...")
	patches, latest, err := catalog.LatestPatches(&http.Client{Timeout: 90 * time.Second}, g.Build)
	if err != nil {
		p.Say("Update check failed:", err)
		return p.Yes("Continue anyway?", false)
	}
	if len(patches) == 0 {
		cache.Cleanup(gamefiles.CacheDir(g.Root), g.Build)
		p.Sayf("Build %d is up to date.\n", g.Build)
		return true
	}
	p.Sayf("Update available: %d -> %d (%d patches).\n", g.Build, latest, len(patches))
	if !p.Yes("Install now?", true) {
		return p.Yes("Start the game without updating?", false)
	}
	if err := gamefiles.InstallPatches(g.Root, patches); err != nil {
		p.Say("Update failed:", err)
		return false
	}
	updated, err := gamefiles.Open(g.Root)
	if err != nil {
		p.Say("After update:", err)
		return false
	}
	p.Notify(fmt.Sprintf("VK Play is updated to build %d", updated.Build))
	*g = updated
	return true
}

func printAccounts(cfg *config.Config, def config.Account, hasDef bool) {
	if len(cfg.Accounts) == 0 {
		fmt.Println("\nNo saved accounts. Type + to add one.")
		return
	}
	fmt.Println("\nAccounts:")
	width, service := 0, 0
	for _, a := range cfg.Accounts {
		width = max(width, len([]rune(a.Label())))
		service = max(service, len(a.Service()))
	}
	for i, a := range cfg.Accounts {
		mark := ""
		if hasDef && a.UserID == def.UserID {
			mark = "  <- last used"
		}
		fmt.Printf("  %d) %-*s  %-*s%s\n", i+1, width, a.Label(), service, a.Service(), mark)
	}
}

func printMenu(cfg *config.Config, def config.Account, hasDef bool) {
	fmt.Println()
	if hasDef {
		fmt.Printf("  Enter    play as %s\n", def.Label())
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
	if platform.GameRunning() {
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

func fxAccountFor(cfg *config.Config, def config.Account, hasDef bool) (config.Account, bool) {
	if hasDef && def.IsFX() {
		return def, true
	}
	for _, a := range cfg.Accounts {
		if a.IsFX() {
			return a, true
		}
	}
	return config.Account{}, false
}

func (s *session) pickFXAccount(arg string, def config.Account, hasDef bool) (config.Account, bool) {
	cfg := s.cfg.Get()
	if arg != "" {
		n, err := strconv.Atoi(arg)
		if err != nil || n < 1 || n > len(cfg.Accounts) || !cfg.Accounts[n-1].IsFX() {
			s.p.Say("That number is not an FX ID account.")
			return config.Account{}, false
		}
		return cfg.Accounts[n-1], true
	}
	fallback, ok := fxAccountFor(&cfg, def, hasDef)
	if !ok {
		s.p.Say("Add an FX ID account first.")
		return config.Account{}, false
	}
	var fx []int
	for i, a := range cfg.Accounts {
		if a.IsFX() {
			fx = append(fx, i)
		}
	}
	if len(fx) == 1 {
		return fallback, true
	}
	s.p.Say("Which FX ID account?")
	for _, i := range fx {
		s.p.Sayf("  %d  %s\n", i+1, cfg.Accounts[i].Label())
	}
	answer := firstWord(s.p.line("Account number (Enter for " + fallback.Label() + "): "))
	if answer == "" {
		return fallback, true
	}
	return s.pickFXAccount(answer, def, hasDef)
}

func (s *session) pickAccount(arg string, def config.Account, hasDef bool) (config.Account, bool) {
	cfg := s.cfg.Get()
	switch {
	case len(cfg.Accounts) == 0:
		return config.Account{}, false
	case arg == "" && len(cfg.Accounts) == 1:
		return cfg.Accounts[0], true
	case arg == "":
		prompt := "Account number: "
		if hasDef {
			prompt = "Account number (Enter for " + def.Label() + "): "
		}
		arg = firstWord(s.p.line(prompt))
		if arg == "" && hasDef {
			return def, true
		}
	}
	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > len(cfg.Accounts) {
		s.p.Say("No such account number.")
		return config.Account{}, false
	}
	return cfg.Accounts[n-1], true
}

func (s *session) rename(acc config.Account) {
	name := strings.TrimSpace(s.p.line("New name for " + acc.Label() + " (Enter to show the login): "))
	if err := s.cfg.UpdateAccount(acc.UserID, func(a *config.Account) { a.Name = name }); err != nil {
		s.p.Say("Error:", err)
	}
}

func (s *session) showBranches(acc config.Account) {
	s.p.Sayf("Asking FX ID for branches of %s...\n", acc.Label())
	branches, err := fxBranches(s.client, acc)
	if errors.Is(err, errNeedLogin) {
		relogged, lerr := s.relogin(acc)
		if lerr != nil {
			s.p.Say("Sign-in failed:", lerr)
			return
		}
		acc = relogged
		branches, err = fxBranches(s.client, acc)
	}
	if err != nil {
		s.p.Say("Could not list branches:", err)
		return
	}
	printFXBranches(s.p, branches)
	s.chooseBranch(acc, branches)
}

func (s *session) chooseBranch(acc config.Account, branches []fxBranchInfo) {
	var open []string
	for _, b := range branches {
		if b.Err == nil {
			open = append(open, b.Name)
		}
	}
	if len(open) == 0 || (len(open) == 1 && open[0] == gamefiles.DefaultBranch && acc.Branch == "") {
		return
	}
	current := acc.Branch
	if current == "" {
		current = gamefiles.DefaultBranch
	}
	s.p.Sayf("Which branch should %s play?\n", acc.Label())
	for i, name := range open {
		mark := ""
		if name == current {
			mark = "  <- current"
		}
		s.p.Sayf("  %d  %s%s\n", i+1, name, mark)
	}
	n, err := strconv.Atoi(firstWord(s.p.line("Branch number (Enter to keep " + current + "): ")))
	if err != nil || n < 1 || n > len(open) {
		return
	}
	acc.Branch = open[n-1]
	if acc.Branch == gamefiles.DefaultBranch {
		acc.Branch = ""
	}
	if err := s.cfg.UpdateAccount(acc.UserID, func(a *config.Account) { a.Branch = acc.Branch }); err != nil {
		s.p.Say("Error:", err)
		return
	}
	if acc.Branch == "" {
		s.p.Say("The account will play the main client.")
	} else {
		s.p.Sayf("The account will play %s. It is installed next to the main game on the next start.\n", acc.Branch)
	}
}

func (s *session) activateKey(acc config.Account) {
	key := strings.TrimSpace(s.p.line("Key for a closed branch (Enter to skip): "))
	if key == "" {
		return
	}
	branch, err := fxActivateKey(s.client, acc, key)
	if errors.Is(err, errNeedLogin) {
		relogged, lerr := s.relogin(acc)
		if lerr != nil {
			s.p.Say("Sign-in failed:", lerr)
			return
		}
		acc = relogged
		branch, err = fxActivateKey(s.client, acc, key)
	}
	if err != nil {
		s.p.Say("Key not activated:", err)
		return
	}
	s.p.Sayf("Key activated: branch %q is now available to %s.\n", branch, acc.Label())
	s.showBranches(acc)
}

func (s *session) addAccount() (config.Account, error) {
	p, client, cfg := s.p, s.client, s.cfg
	p.Say("Which service is the account on?")
	p.Say("  1  VK Play (sign-in in the browser)")
	p.Say("  2  FX ID, Wishlist Games (e-mail and a code)")
	var added config.Account
	var err error
	switch firstWord(p.line("> ")) {
	case "1":
		added, err = loginAccount(client, cfg, "", vkSignInTimeout)
	case "2":
		added, err = loginFX(p, client, cfg, "", "")
	default:
		return config.Account{}, errors.New("no service chosen")
	}
	if err != nil {
		return config.Account{}, err
	}
	if name := p.line("Account name (Enter to keep " + added.Label() + "): "); name != "" {
		added.Name = name
		if err := cfg.UpdateAccount(added.UserID, func(a *config.Account) { a.Name = name }); err != nil {
			return config.Account{}, err
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
	cfg    *config.Store
	client *http.Client
	found  *foundGame
}

type foundGame struct{ game *gamefiles.Install }

func (s *session) with(p prompter) *session {
	c := *s
	c.p = p
	return &c
}

func (s *session) vkGame() (gamefiles.Install, error) {
	if s.found.game == nil {
		g, err := chooseGameDir(s.p, s.cfg)
		if err != nil {
			return gamefiles.Install{}, err
		}
		if s.cfg.Get().Game != g.Root {
			if err := s.cfg.Update(func(c *config.Config) { c.Game = g.Root }); err != nil {
				return gamefiles.Install{}, err
			}
		}
		s.p.Sayf("Game: %s (build %d)\n", g.Root, g.Build)
		if !ensureUpdated(s.p, &g) {
			return gamefiles.Install{}, errQuit
		}
		s.found.game = &g
	}
	return *s.found.game, nil
}

func (s *session) vkRoot() (string, error) {
	cfg := s.cfg.Get()
	if cfg.Game != "" {
		return cfg.Game, nil
	}
	s.p.Say("Choose a folder for the VK Play client. Press Enter for the default, or q to return to the menu.")
	for {
		dir := strings.Trim(s.p.line(fmt.Sprintf("VK Play game folder [%s]: ", platform.DefaultGameDir())), `"' `)
		if dir == "" {
			if s.p.ask != nil {
				return "", errQuit
			}
			dir = platform.DefaultGameDir()
		} else if strings.EqualFold(dir, "q") {
			return "", errQuit
		}
		root, err := filepath.Abs(dir)
		if err != nil {
			s.p.Say("Invalid folder:", err)
			continue
		}
		if _, ok := gamefiles.ReadBranchState(root); ok || strings.EqualFold(root, cfg.FXGame) {
			s.p.Say("That folder contains an FX ID client. Choose another folder.")
			continue
		}
		if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 && !gamefiles.IsVKInstall(root) && !s.p.Yes("The folder is not empty. Continue there?", false) {
			continue
		}
		if err := s.cfg.Update(func(c *config.Config) { c.Game = root }); err != nil {
			return "", err
		}
		return root, nil
	}
}

func (s *session) fxRoot() (string, error) {
	cfg := s.cfg.Get()
	if cfg.FXGame != "" {
		return cfg.FXGame, nil
	}
	defaultDir := config.DefaultFXDir()
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
			s.p.Say("Invalid folder:", err)
			continue
		}
		if strings.EqualFold(root, cfg.Game) || gamefiles.IsVKInstall(root) {
			s.p.Say("Choose a folder different from the VK Play client.")
			continue
		}
		if state, ok := gamefiles.ReadBranchState(root); ok && state.Branch != gamefiles.DefaultBranch {
			s.p.Say("That folder contains a closed FX ID branch.")
			continue
		}
		if err := s.cfg.Update(func(c *config.Config) { c.FXGame = root }); err != nil {
			return "", err
		}
		return root, nil
	}
}

func (s *session) play(acc config.Account) error {
	switch {
	case acc.IsFX() && acc.Branch != "":
		cfg := s.cfg.Get()
		if cfg.FXGame == "" && cfg.BranchGames[strings.ToLower(acc.Branch)] == "" {
			if _, err := s.fxRoot(); err != nil {
				return err
			}
			cfg = s.cfg.Get()
		}
		return playFXInstall(s.client, &s.p, acc, acc.Branch, cfg.BranchDir(acc.Branch), cfg.FXGame, false, cfg.AllowMods)
	case acc.IsFX():
		root, err := s.fxRoot()
		if err != nil {
			return err
		}
		return playFXInstall(s.client, &s.p, acc, gamefiles.DefaultBranch, root, "", true, s.cfg.Get().AllowMods)
	}
	g, err := s.vkGame()
	if err != nil {
		return err
	}
	if err := gamefiles.VerifyBeforeLaunch(s.p, g, s.cfg.Get().AllowMods); err != nil {
		return err
	}
	return startGame(s.client, g, acc)
}

func (s *session) relogin(acc config.Account) (config.Account, error) {
	s.p.Sayf("Session for %s has expired, sign in again.\n", acc.Label())
	var relogged config.Account
	var err error
	if acc.IsFX() {
		relogged, err = loginFX(s.p, s.client, s.cfg, acc.Email, acc.Name)
	} else {
		relogged, err = loginAccount(s.client, s.cfg, acc.Name, vkSignInTimeout)
	}
	if err != nil {
		return config.Account{}, err
	}
	if relogged.UserID != acc.UserID {
		s.p.Sayf("Signed in to a different account (%s); it was added to the list.\n", relogged.Label())
	}
	return relogged, nil
}

func runInteractive() error {
	loaded, err := config.Load()
	if err != nil {
		return err
	}
	store := config.NewStore(loaded)
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
		cfg := store.Get()
		def, hasDef := cfg.DefaultAccount()
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
		var acc config.Account
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
			if !p.Yes("Close Armored Warfare? If you are in a battle, you leave it.", false) {
				continue
			}
			fmt.Println("Closing the game...")
			if err := platform.CloseGame(10 * time.Second); err != nil {
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
			if !p.Yes("Start the game with this account?", true) {
				continue
			}
			acc, _ = store.Find(added.UserID)
		case strings.HasPrefix(answer, "-"):
			n, err := strconv.Atoi(answer[1:])
			if err != nil || n < 1 || n > len(cfg.Accounts) {
				fmt.Println("No such account number.")
				continue
			}
			victim := cfg.Accounts[n-1]
			if !p.Yes("Remove account "+victim.Label()+"?", false) {
				continue
			}
			if victim.IsFX() {
				err = config.ClearRefreshToken(victim.UserID)
			} else {
				err = dropSession(victim.UserID)
			}
			if err != nil {
				fmt.Println("Error:", err)
			}
			if err := store.Update(func(c *config.Config) { c.Remove(victim.UserID) }); err != nil {
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
		fmt.Printf("AWLauncher - %s\n\n", acc.Label())
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
		if err := store.Update(func(c *config.Config) { c.LastUserID = acc.UserID }); err != nil {
			return err
		}
		if platform.OwnsConsole() {
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
