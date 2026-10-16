package platform

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0755); err != nil {
		t.Fatal(err)
	}
}

// fakeDeck lays out a Steam Deck home: Steam with Valve's Proton builds and
// the sniper runtime, GE-Proton in compatibilitytools.d and a library on the SD card.
func fakeDeck(t *testing.T) (home, card string) {
	home, card = t.TempDir(), t.TempDir()
	steam := filepath.Join(home, ".local", "share", "Steam")
	if err := os.MkdirAll(filepath.Join(home, ".steam"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(steam, filepath.Join(home, ".steam", "root")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(steam, "steamapps", "libraryfolders.vdf"), `"libraryfolders"
{
	"0" { "path" "`+steam+`" }
	"1" { "path" "`+card+`" }
}`)
	manifest := `"manifest" { "commandline" "/proton %verb%" "require_tool_appid" "1628350" }`
	for _, dir := range []string{
		filepath.Join(steam, "steamapps", "common", "Proton 9.0 (Beta)"),
		filepath.Join(steam, "steamapps", "common", "Proton - Experimental"),
		filepath.Join(card, "steamapps", "common", "Proton 10.0"),
		filepath.Join(steam, "compatibilitytools.d", "GE-Proton9-27"),
		filepath.Join(steam, "compatibilitytools.d", "GE-Proton10-15"),
	} {
		writeFile(t, filepath.Join(dir, "proton"), "#!/usr/bin/env python3\n")
		writeFile(t, filepath.Join(dir, "toolmanifest.vdf"), manifest)
	}
	writeFile(t, filepath.Join(steam, "compatibilitytools.d", "broken", "README"), "")
	writeFile(t, filepath.Join(card, "steamapps", "appmanifest_1628350.acf"), `"AppState" { "appid" "1628350" "installdir" "SteamLinuxRuntime_sniper" }`)
	writeFile(t, filepath.Join(card, "steamapps", "common", "SteamLinuxRuntime_sniper", "_v2-entry-point"), "#!/bin/sh\n")
	return home, card
}

func noCommands(string) (string, error) { return "", errors.New("not found") }

func names(protons []Proton) []string {
	var out []string
	for _, p := range protons {
		out = append(out, p.Name)
	}
	return out
}

func envValue(env []string, key string) string {
	value := ""
	for _, kv := range env {
		if k, v, _ := strings.Cut(kv, "="); k == key {
			value = v
		}
	}
	return value
}

func clearRunnerEnv(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	for _, key := range []string{"WINEPREFIX", "AWLAUNCHER_RUNNER", "GAMEID", "STORE", "PROTONPATH", "STEAM_COMPAT_DATA_PATH", "STEAM_COMPAT_CLIENT_INSTALL_PATH", "STEAM_COMPAT_INSTALL_PATH", "STEAM_COMPAT_TOOL_PATHS", "STEAM_COMPAT_MOUNTS"} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
	SetProton("")
	t.Cleanup(func() { SetProton("") })
}

func TestFindProtonsPrefersValve(t *testing.T) {
	home, _ := fakeDeck(t)
	got := names(findProtons(home))
	want := []string{"Proton 10.0", "Proton 9.0 (Beta)", "GE-Proton10-15", "GE-Proton9-27", "Proton - Experimental"}
	if !slices.Equal(got, want) {
		t.Fatalf("findProtons = %q, want %q", got, want)
	}
}

func TestProtonRunsInSteamRuntime(t *testing.T) {
	clearRunnerEnv(t)
	home, card := fakeDeck(t)
	game := filepath.Join(card, "Games", "ArmoredWarfare")
	r, err := gameRunner(home, game, noCommands)
	if err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(card, "steamapps", "common", "SteamLinuxRuntime_sniper", "_v2-entry-point")
	proton := filepath.Join(card, "steamapps", "common", "Proton 10.0")
	want := []string{entry, "--verb=waitforexitandrun", "--", filepath.Join(proton, "proton"), "waitforexitandrun"}
	if !slices.Equal(r.argv, want) {
		t.Fatalf("argv = %q, want %q", r.argv, want)
	}
	prefix := envValue(r.env, "STEAM_COMPAT_DATA_PATH")
	if prefix == "" || envValue(r.env, "WINEPREFIX") != prefix {
		t.Fatalf("prefix: STEAM_COMPAT_DATA_PATH=%q WINEPREFIX=%q", prefix, envValue(r.env, "WINEPREFIX"))
	}
	if link, err := os.Readlink(filepath.Join(prefix, "pfx")); err != nil || link != "." {
		t.Fatalf("pfx = %q, %v", link, err)
	}
	if got := envValue(r.env, "STEAM_COMPAT_MOUNTS"); !strings.Contains(got, game) {
		t.Fatalf("STEAM_COMPAT_MOUNTS = %q, want the game folder", got)
	}
	if got := envValue(r.env, "STEAM_COMPAT_INSTALL_PATH"); got != game {
		t.Fatalf("STEAM_COMPAT_INSTALL_PATH = %q", got)
	}
	if !strings.Contains(r.name, "Proton 10.0") {
		t.Fatalf("name = %q", r.name)
	}
	if filepath.Base(prefix) != "Proton-10.0" || filepath.Base(filepath.Dir(prefix)) != "prefixes" {
		t.Fatalf("each Proton build should get its own prefix, got %s", prefix)
	}
}

func TestChosenProton(t *testing.T) {
	clearRunnerEnv(t)
	home, card := fakeDeck(t)
	chosen := filepath.Join(home, ".local", "share", "Steam", "compatibilitytools.d", "GE-Proton9-27")
	SetProton(chosen)
	r, err := gameRunner(home, card, noCommands)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(r.argv, filepath.Join(chosen, "proton")) {
		t.Fatalf("argv = %q, want %s", r.argv, chosen)
	}
	if filepath.Base(envValue(r.env, "WINEPREFIX")) != "GE-Proton9-27" {
		t.Fatalf("prefix = %q", envValue(r.env, "WINEPREFIX"))
	}
	SetProton(filepath.Join(card, "gone"))
	if r, _ = gameRunner(home, card, noCommands); !strings.Contains(r.name, "Proton 10.0") {
		t.Fatalf("a missing choice should fall back to the best build, got %q", r.name)
	}
}

func TestUmuGetsTheBestProton(t *testing.T) {
	clearRunnerEnv(t)
	home, _ := fakeDeck(t)
	writeFile(t, filepath.Join(home, ".config", "heroic", "tools", "runtimes", "umu", "umu_run.py"), "")
	look := func(name string) (string, error) {
		if name == "python3" {
			return "/usr/bin/python3", nil
		}
		return "", errors.New("not found")
	}
	r, err := gameRunner(home, home, look)
	if err != nil {
		t.Fatal(err)
	}
	if r.argv[0] != "/usr/bin/python3" || !strings.HasSuffix(r.argv[1], "umu_run.py") {
		t.Fatalf("argv = %q", r.argv)
	}
	if got := envValue(r.env, "GAMEID"); got != "umu-443110" {
		t.Fatalf("GAMEID = %q", got)
	}
	if got := envValue(r.env, "PROTONPATH"); filepath.Base(got) != "Proton 10.0" {
		t.Fatalf("PROTONPATH = %q", got)
	}
}

func TestRunnerFallsBackToWine(t *testing.T) {
	clearRunnerEnv(t)
	home := t.TempDir()
	if _, err := gameRunner(home, home, noCommands); err == nil {
		t.Fatal("expected an error with nothing installed")
	}
	look := func(name string) (string, error) {
		if name == "wine" {
			return "/usr/bin/wine", nil
		}
		return "", errors.New("not found")
	}
	r, err := gameRunner(home, home, look)
	if err != nil || !slices.Equal(r.argv, []string{"/usr/bin/wine"}) {
		t.Fatalf("argv = %q, %v", r.argv, err)
	}
	t.Setenv("AWLAUNCHER_RUNNER", "/opt/run --flag")
	if r, _ = gameRunner(home, home, look); !slices.Equal(r.argv, []string{"/opt/run", "--flag"}) {
		t.Fatalf("custom argv = %q", r.argv)
	}
}

func TestUmuInLocalBin(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".local", "bin", "umu-run"), "#!/bin/sh\n")
	if got := findUmu(home, noCommands); len(got) != 1 || got[0] != filepath.Join(home, ".local", "bin", "umu-run") {
		t.Fatalf("findUmu = %q", got)
	}
}
