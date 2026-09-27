package region

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	vkUserCfg = "net_frontline_address = awauth.arwar.ru\r\ng_language = Russian\r\n"
	fxUserCfg = "net_frontline_address = 95.211.7.247\r\ng_language = English\r\n"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestSwitchPutsEachServicesOwnFile(t *testing.T) {
	root := t.TempDir()
	p := pathsOf(root)
	write(t, p.root, vkUserCfg)
	if err := Ensure(root, VK); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.dir); err == nil {
		t.Fatal("state folder created without a switch")
	}
	if Ensure(root, FX) == nil {
		t.Fatal("switched to FX ID without its user.cfg")
	}
	write(t, FXConfigPath(root), fxUserCfg)
	for i := 0; i < 2; i++ {
		if err := Ensure(root, FX); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, p.root); got != fxUserCfg || Current(root) != FX {
			t.Fatalf("pass %d: user.cfg = %q, region %s", i, got, Current(root))
		}
	}
	if err := Ensure(root, VK); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, p.root); got != vkUserCfg || Current(root) != VK {
		t.Fatalf("restored user.cfg = %q, region %s", got, Current(root))
	}
	if _, err := os.Stat(p.vk); err == nil {
		t.Fatal("the VK Play copy is left behind")
	}
	if readFile(t, FXConfigPath(root)) != fxUserCfg {
		t.Fatal("the FX ID user.cfg is lost")
	}
}

func TestFXInstallIsNotSavedAsVK(t *testing.T) {
	root := t.TempDir()
	p := pathsOf(root)
	write(t, p.root, fxUserCfg)
	write(t, FXConfigPath(root), fxUserCfg)
	if err := Ensure(root, FX); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.vk); err == nil {
		t.Fatal("the FX ID file was kept as the VK Play one")
	}
	if err := Ensure(root, VK); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.root); err == nil {
		t.Fatal("VK Play got the FX ID user.cfg; it must come from the VK Play files")
	}
}

func TestInterruptedSwitchFinishes(t *testing.T) {
	root := t.TempDir()
	p := pathsOf(root)
	write(t, p.root, vkUserCfg)
	write(t, p.vk, vkUserCfg)
	write(t, p.marker, FX)
	write(t, FXConfigPath(root), fxUserCfg)
	if err := Ensure(root, FX); err != nil {
		t.Fatal(err)
	}
	if readFile(t, p.root) != fxUserCfg || readFile(t, p.vk) != vkUserCfg {
		t.Fatal("interrupted switch to FX ID not finished")
	}
}
