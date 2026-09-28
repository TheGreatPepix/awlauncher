package startup

import (
	"errors"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/gui/update"
	"golang.org/x/sys/windows/registry"
)

const (
	runKey      = `Software\Microsoft\Windows\CurrentVersion\Run`
	approvedKey = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`
	runValue    = "AWLauncher"

	autostartFlag = "--autostart"
	trayFlag      = "--tray"
)

const (
	autostartOff    = "off"
	autostartWindow = "window"
	autostartTray   = "tray"
)

func hasArg(args []string, flag string) bool {
	for _, a := range args[1:] {
		if strings.EqualFold(a, flag) {
			return true
		}
	}
	return false
}

func runCommand(exe string, tray bool) string {
	cmd := `"` + exe + `" ` + autostartFlag
	if tray {
		cmd += " " + trayFlag
	}
	return cmd
}

func readRunValue() (string, bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer k.Close()
	v, _, err := k.GetStringValue(runValue)
	return v, err == nil
}

func startupDisabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, approvedKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	data, _, err := k.GetBinaryValue(runValue)
	return err == nil && len(data) > 0 && data[0]&1 == 1
}

func Mode() string {
	v, ok := readRunValue()
	switch {
	case !ok || startupDisabled():
		return autostartOff
	case strings.Contains(strings.ToLower(v), trayFlag):
		return autostartTray
	}
	return autostartWindow
}

func Set(mode string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if mode == autostartOff {
		if err := k.DeleteValue(runValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}
	if mode != autostartWindow && mode != autostartTray {
		return errors.New("unknown autostart mode " + mode)
	}
	exe, err := update.SelfPath()
	if err != nil {
		return err
	}
	if err := k.SetStringValue(runValue, runCommand(exe, mode == autostartTray)); err != nil {
		return err
	}

	if a, err := registry.OpenKey(registry.CURRENT_USER, approvedKey, registry.SET_VALUE); err == nil {
		_ = a.DeleteValue(runValue)
		a.Close()
	}
	return nil
}

func Refresh() {
	v, ok := readRunValue()
	exe, err := update.SelfPath()
	if !ok || err != nil || strings.Contains(strings.ToLower(v), strings.ToLower(`"`+exe+`"`)) {
		return
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	_ = k.SetStringValue(runValue, runCommand(exe, strings.Contains(strings.ToLower(v), trayFlag)))
}

func TrayRequested(args []string) bool { return hasArg(args, trayFlag) }
