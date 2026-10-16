package gui

import (
	"log"

	"github.com/TheGreatPepix/awlauncher/internal/launcher"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
)

type signIn struct {
	launcher.VKSignIn
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
		g.emit(map[string]any{"type": "signin", "active": true})
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
	g.emitSignIn()
}
