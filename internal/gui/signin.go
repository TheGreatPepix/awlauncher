package gui

import (
	"log"

	"github.com/TheGreatPepix/awlauncher/internal/launcher"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
)

type signIn struct {
	launcher.VKSignIn
	window bool
}

func (g *App) vkSignInStarted(s launcher.VKSignIn) func() {
	g.host.post(func() {
		g.endSignIn()
		g.signIn = &signIn{VKSignIn: s}
		g.emitSignIn()
	})
	return func() { g.host.post(g.endSignIn) }
}

func (g *App) emitSignIn() {
	if s := g.signIn; s != nil {
		g.emit(map[string]any{"type": "signin", "active": true, "window": s.window})
	} else {
		g.emit(map[string]any{"type": "signin", "active": false})
	}
}

func (g *App) signInCommand(cmd string) {
	s := g.signIn
	if s == nil {
		return
	}
	switch cmd {
	case "signinOpen":
		if err := platform.OpenBrowser(s.URL); err != nil {
			log.Print("Error: ", err)
		}
	case "signinFresh":
		if err := platform.OpenBrowser(s.FreshURL); err != nil {
			log.Print("Error: ", err)
		} else {
			log.Print("Sign in with the other account in the browser; the launcher keeps waiting.")
		}
	case "signinHere":
		if s.window {
			g.host.showSignIn()
			return
		}
		report := func(code, state string) { s.Report(code, state, nil) }
		closed := func() {
			if s.window {
				s.window = false
				if g.signIn == s {
					g.emitSignIn()
				}
			}
		}
		if err := g.host.openSignIn(s.FreshURL, report, closed); err != nil {
			log.Print("Cannot open the sign-in window: ", err)
			return
		}
		s.window = true
		log.Print("VK Play sign-in opened in the launcher window.")
		g.emitSignIn()
	case "signinCancel":
		s.Report("", "", launcher.ErrCancelled)
	}
}

func (g *App) endSignIn() {
	s := g.signIn
	if s == nil {
		return
	}
	g.signIn = nil
	if s.window {
		s.window = false
		g.host.closeSignIn()
	}
	g.emitSignIn()
}
