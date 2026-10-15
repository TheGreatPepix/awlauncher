// Command awcore is the launcher without a window. The AWLauncher window
// starts it and talks to it with JSON lines on its standard input and output.
package main

import (
	"flag"
	"os"

	"github.com/TheGreatPepix/awlauncher/internal/gui"
)

func main() {
	app := flag.String("app", "", "the window's executable, which starts with Windows")
	flag.Parse()
	os.Exit(gui.RunCore(os.Stdin, os.Stdout, *app))
}
