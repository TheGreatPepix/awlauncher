package progress

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func terminal(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	return err == nil
}

func ClearScreen() {
	if !ui.live || !terminal(os.Stdin) {
		return
	}
	fmt.Print("\x1b[2J\x1b[H")
}

func InteractiveScreen() bool { return ui.live && terminal(os.Stdin) }

func enableVT() bool { return terminal(os.Stdout) && os.Getenv("TERM") != "dumb" }

func consoleWidth() int {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col <= 20 {
		return 80
	}
	return int(ws.Col)
}
