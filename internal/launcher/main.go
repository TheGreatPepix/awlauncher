package launcher

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
)

func Run() int {
	run := runInteractive
	if args := os.Args[1:]; len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "play":
			run = func() error { return runPlay(strings.Join(args[1:], " ")) }
		case "version", "--version", "-v":
			fmt.Println("AWLauncher", Version)
			return 0
		case "help", "--help", "-h", "/?":
			printUsage()
			return 0
		default:
			fmt.Fprintf(os.Stderr, "Unknown command %q.\n\n", args[0])
			printUsage()
			return 2
		}
	}
	bringLauncherForward = focusConsole
	var err error
	acquired, lockErr := acquireInstance()
	if lockErr != nil {
		err = fmt.Errorf("cannot lock the launcher instance: %w", lockErr)
	} else if acquired {
		err = run()
	} else {
		if activateRunningGUI() {
			fmt.Println("AWLauncher is already running; its window is brought forward.")
		}
		err = errors.New(instanceBusyMessage())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", strings.TrimSpace(err.Error()))
	}
	if platform.OwnsConsole() {
		if err != nil {
			fmt.Print("\nPress Enter to close this window...")
			_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		} else {
			time.Sleep(3 * time.Second)
		}
	}
	if err != nil {
		return 1
	}
	return 0
}
