//go:build windows

package main

import (
	"os"
	"runtime"

	"github.com/TheGreatPepix/awlauncher/internal/gui"
)

func init() { runtime.LockOSThread() }

func main() { os.Exit(gui.Run()) }
