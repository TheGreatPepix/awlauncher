package launcher

import "testing"

func TestAutostartCommand(t *testing.T) {
	if got := runCommand(`C:\Apps\AWLauncher\AWLauncher.exe`, false); got != `"C:\Apps\AWLauncher\AWLauncher.exe" --autostart` {
		t.Errorf("window: %s", got)
	}
	if got := runCommand(`C:\Apps\AWLauncher\AWLauncher.exe`, true); got != `"C:\Apps\AWLauncher\AWLauncher.exe" --autostart --tray` {
		t.Errorf("tray: %s", got)
	}
	if !hasArg([]string{"AWLauncher.exe", "--autostart", "--TRAY"}, trayFlag) {
		t.Error("the tray flag is missed")
	}
	if hasArg([]string{"--tray"}, trayFlag) || hasArg([]string{"AWLauncher.exe", "--updated", "12"}, trayFlag) {
		t.Error("the tray flag is found where it is not")
	}
	if err := setAutostart("sometimes"); err == nil {
		t.Error("an unknown mode is accepted")
	}
}
