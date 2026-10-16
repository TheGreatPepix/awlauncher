package gui

import (
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/gui/update"
	"github.com/TheGreatPepix/awlauncher/internal/launcher"
)

func (g *App) updateSelf() {
	if ops := g.ops.List(); len(ops) > 0 {
		g.notice("Wait until “" + ops[0].Title + "” finishes")
		return
	}
	if g.core {
		g.updateThroughSetup()
		return
	}
	op := launcher.Operation{Title: "Updating AWLauncher", Game: true}
	g.run(op, "The update is installed. It takes effect on the next start.", func(*launcher.Session) (bool, error) {
		exe, err := update.Install(&http.Client{Timeout: 10 * time.Minute}, launcher.Version)
		if err != nil {
			return false, err
		}
		g.host.post(func() {
			if len(g.ops.List()) > 1 {
				log.Print("The update takes effect when AWLauncher starts next time.")
				return
			}
			if err := update.StartUpdated(exe, func(pid int) { procAllowSetForegroundWindo.Call(uintptr(pid)) }); err != nil {
				log.Print("Cannot restart AWLauncher: ", err)
				return
			}
			g.host.exit()
		})
		return false, nil
	})
}

// updateThroughSetup updates the Flutter launcher: it downloads the
// installer, starts it for the window's folder and quits, and the installer
// starts the new version when it is done.
func (g *App) updateThroughSetup() {
	if g.appExe == "" {
		g.notice("This build cannot update itself")
		return
	}
	op := launcher.Operation{Title: "Updating AWLauncher", Game: true}
	g.run(op, "", func(*launcher.Session) (bool, error) {
		setup, err := update.DownloadSetup(&http.Client{Timeout: 10 * time.Minute}, launcher.Version)
		if err != nil {
			return false, err
		}
		if err := update.RunSetup(setup, filepath.Dir(g.appExe), false); err != nil {
			return false, err
		}
		log.Print("The installer replaces AWLauncher and starts it again.")
		g.host.post(g.host.exit)
		return false, nil
	})
}
