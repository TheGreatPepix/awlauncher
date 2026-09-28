package gui

import (
	"log"
	"net/http"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/gui/update"
	"github.com/TheGreatPepix/awlauncher/internal/launcher"
)

func (g *App) updateSelf() {
	if ops := g.ops.List(); len(ops) > 0 {
		g.notice("Wait until “" + ops[0].Title + "” finishes")
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
