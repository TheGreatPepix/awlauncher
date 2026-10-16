//go:build windows

// Command awbridge moves AWLauncher 0.5 to the Flutter launcher. Releases
// publish it as AWLauncher.exe, the one file the WebView launcher updates
// itself with. It carries the Flutter launcher's installer: started by the
// old launcher after its update, it waits for that one to quit and installs
// the new version into the same folder, which then starts by itself.
// Started by hand, it opens the installer as usual.
package main

import (
	"embed"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/TheGreatPepix/awlauncher/internal/gui/update"
	"golang.org/x/sys/windows"
)

//go:embed payload
var payload embed.FS

func main() {
	if err := run(); err != nil {
		say("AWLauncher cannot install the new version: " + err.Error() +
			"\n\nDownload AWLauncher-Setup.exe from https://github.com/TheGreatPepix/awlauncher/releases/latest")
		os.Exit(1)
	}
}

func run() error {
	updating := update.WaitForOld(os.Args)
	setup, err := unpackSetup()
	if err != nil {
		return err
	}
	if !updating {
		return exec.Command(setup).Start()
	}
	exe, err := update.SelfPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	if !update.OwnFolder(dir) {
		// A portable launcher on the desktop or in Downloads: the folder
		// would fill with the new launcher's files, so it is installed
		// where the installer puts it. The old one is removed; this one
		// stays and opens the installer if started again.
		update.RemoveLeftovers(exe)
		return update.RunSetup(setup, "", true)
	}
	// The installer writes AWLauncher.exe, which is this program; the new
	// launcher deletes it when it starts.
	_ = update.StepAside(exe)
	return update.RunSetup(setup, dir, true)
}

func unpackSetup() (string, error) {
	data, err := fs.ReadFile(payload, "payload/"+update.SetupAssetName)
	if err != nil {
		return "", fs.ErrNotExist
	}
	dir, err := os.MkdirTemp("", "AWLauncher-update-")
	if err != nil {
		return "", err
	}
	setup := filepath.Join(dir, update.SetupAssetName)
	return setup, os.WriteFile(setup, data, 0o755)
}

func say(text string) {
	title, _ := windows.UTF16PtrFromString("AWLauncher")
	body, _ := windows.UTF16PtrFromString(text)
	windows.MessageBox(0, body, title, windows.MB_OK|windows.MB_ICONERROR)
}
