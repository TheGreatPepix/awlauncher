//go:build !windows

package gui

func (g *App) updateSelf() { g.notice("AWLauncher cannot update itself on this system") }
