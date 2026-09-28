package launcher

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
)

func printUsage() {
	name := filepath.Base(os.Args[0])
	fmt.Printf(`Usage:
  %[1]s                 the menu: accounts, sign-in, install, branches and the game folder
  %[1]s play [ACCOUNT]  update the game, start it for ACCOUNT and wait until it exits
  %[1]s version         print the launcher version

ACCOUNT is a number from the menu, an account name or login; without it the last used account plays.
"play" asks nothing: it installs updates and repairs files, but never a new install or a sign-in;
do those in the menu first. It suits launchers that track the game by its process, like Steam.
`, name)
}

func pickPlayAccount(cfg config.Config, sel string) (config.Account, error) {
	sel = strings.TrimSpace(sel)
	if len(cfg.Accounts) == 0 {
		return config.Account{}, errors.New("there are no accounts; run AWLauncher without arguments and add one")
	}
	if sel == "" {
		if acc, ok := cfg.DefaultAccount(); ok {
			return acc, nil
		}
		return config.Account{}, errors.New("no account was used yet; name one: play ACCOUNT")
	}
	if n, err := strconv.Atoi(sel); err == nil && n >= 1 && n <= len(cfg.Accounts) {
		return cfg.Accounts[n-1], nil
	}
	for _, a := range cfg.Accounts {
		if strings.EqualFold(a.DisplayName(), sel) || strings.EqualFold(a.Login(), sel) {
			return a, nil
		}
	}
	return config.Account{}, fmt.Errorf("there is no account %q", sel)
}

var unattendedDeclines = []string{
	"The game is not installed in",
	"There is no Armored Warfare install in",
	"Install Armored Warfare into",
	"Download the FX ID client",
}

func unattendedPrompter() prompter {
	return prompter{
		ask: func(question string) string {
			fmt.Println(strings.TrimSpace(question), "-> no answer (play asks nothing)")
			return ""
		},
		confirm: func(question string, def bool) bool {
			answer := def
			for _, prefix := range unattendedDeclines {
				if strings.HasPrefix(question, prefix) {
					answer = false
				}
			}
			reply := "no"
			if answer {
				reply = "yes"
			}
			fmt.Println(strings.TrimSpace(question), "->", reply)
			return answer
		},
	}
}

func runPlay(sel string) error {
	loaded, err := config.Load()
	if err != nil {
		return err
	}
	store := config.NewStore(loaded)
	acc, err := pickPlayAccount(store.Get(), sel)
	if err != nil {
		return err
	}
	s := &session{p: unattendedPrompter(), cfg: store, client: authClient(), found: &foundGame{}}
	fmt.Printf("AWLauncher %s - %s\n\n", Version, acc.Label())
	switch err := s.play(acc); {
	case errors.Is(err, errNeedLogin):
		return fmt.Errorf("the sign-in of %s has expired; run AWLauncher without arguments to sign in again", acc.Label())
	case errors.Is(err, errQuit):
		return errors.New("the game is not set up; run AWLauncher without arguments to choose the game folder or install the game")
	case err != nil:
		return err
	}
	if err := store.Update(func(c *config.Config) { c.LastUserID = acc.UserID }); err != nil {
		return err
	}
	return waitForGame(3*time.Minute, 2*time.Second)
}

func waitForGame(start, poll time.Duration) error {
	fmt.Println("Waiting for the game to exit...")
	for deadline := time.Now().Add(start); !platform.GameRunning(); time.Sleep(poll) {
		if time.Now().After(deadline) {
			return errors.New("the game did not appear; it may have failed to start")
		}
	}
	for platform.GameRunning() {
		time.Sleep(poll)
	}
	fmt.Println("The game has exited.")
	return nil
}
