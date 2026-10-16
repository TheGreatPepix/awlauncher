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

// fakeDeck lays out a Steam Deck home: Steam with Valve's Proton builds,
// GE-Proton in compatibilitytools.d and a library on the SD card.
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
	for _, dir := range []string{
		filepath.Join(steam, "steamapps", "common", "Proton 9.0 (Beta)"),
		filepath.Join(steam, "steamapps", "common", "Proton - Experimental"),
		filepath.Join(card, "steamapps", "common", "Proton 10.0"),
		filepath.Join(card, "steamapps", "common", "Proton 11.0"),
		filepath.Join(steam, "compatibilitytools.d", "GE-Proton9-27"),
		filepath.Join(steam, "compatibilitytools.d", "GE-Proton10-15"),
	} {
		writeFile(t, filepath.Join(dir, "proton"), "#!/usr/bin/env python3\n")
	}
	writeFile(t, filepath.Join(steam, "compatibilitytools.d", "broken", "README"), "")
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
	for _, key := range []string{"WINEPREFIX", "AWLAUNCHER_RUNNER", "GAMEID", "STORE", "PROTONPATH"} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
	SetProton("")
	t.Cleanup(func() { SetProton("") })
}

func TestFindProtonsPrefersValve(t *testing.T) {
	home, _ := fakeDeck(t)
	got := names(findProtons(home))
	want := []string{"Proton 10.0", "Proton 11.0", "Proton 9.0 (Beta)", "GE-Proton10-15", "GE-Proton9-27", "Proton - Experimental"}
	if !slices.Equal(got, want) {
		t.Fatalf("findProtons = %q, want %q", got, want)
	}
}

func withUmu(t *testing.T, home string) func(string) (string, error) {
	t.Helper()
	writeFile(t, filepath.Join(home, ".local", "bin", "umu-run"), "#!/bin/sh\n")
	return noCommands
}

func TestUmuRunsTheBestProton(t *testing.T) {
	clearRunnerEnv(t)
	home, card := fakeDeck(t)
	r, err := gameRunner(home, withUmu(t, home))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.argv, []string{filepath.Join(home, ".local", "bin", "umu-run")}) {
		t.Fatalf("argv = %q", r.argv)
	}
	if got := envValue(r.env, "PROTONPATH"); got != filepath.Join(card, "steamapps", "common", "Proton 10.0") {
		t.Fatalf("PROTONPATH = %q", got)
	}
	if got := envValue(r.env, "GAMEID"); got != "umu-default" {
		t.Fatalf("GAMEID = %q", got)
	}
	prefix := envValue(r.env, "WINEPREFIX")
	if prefix != r.prefix || filepath.Base(prefix) != "Proton-10.0" || filepath.Base(filepath.Dir(prefix)) != "prefixes" {
		t.Fatalf("each Proton build should get its own prefix, got %s", prefix)
	}
}

func TestChosenProton(t *testing.T) {
	clearRunnerEnv(t)
	home, _ := fakeDeck(t)
	look := withUmu(t, home)
	chosen := filepath.Join(home, ".local", "share", "Steam", "compatibilitytools.d", "GE-Proton9-27")
	SetProton(chosen)
	r, err := gameRunner(home, look)
	if err != nil || envValue(r.env, "PROTONPATH") != chosen || filepath.Base(r.prefix) != "GE-Proton9-27" {
		t.Fatalf("PROTONPATH = %q, prefix %q, %v", envValue(r.env, "PROTONPATH"), r.prefix, err)
	}
	SetProton(filepath.Join(home, "gone"))
	if r, _ = gameRunner(home, look); !strings.Contains(r.name, "Proton 10.0") {
		t.Fatalf("a missing choice should fall back to the best build, got %q", r.name)
	}
}

func TestOwnProtonPath(t *testing.T) {
	clearRunnerEnv(t)
	home, _ := fakeDeck(t)
	t.Setenv("PROTONPATH", "/opt/proton")
	r, err := gameRunner(home, withUmu(t, home))
	if err != nil || envValue(r.env, "PROTONPATH") != "/opt/proton" || filepath.Base(r.prefix) != "prefix" {
		t.Fatalf("PROTONPATH = %q, prefix %q, %v", envValue(r.env, "PROTONPATH"), r.prefix, err)
	}
}

func TestRunnerNeedsUmu(t *testing.T) {
	clearRunnerEnv(t)
	home, _ := fakeDeck(t)
	if _, err := gameRunner(home, noCommands); err == nil || !strings.Contains(err.Error(), "umu-launcher") {
		t.Fatalf("without umu: %v", err)
	}
	t.Setenv("AWLAUNCHER_RUNNER", "/opt/run --flag")
	if r, _ := gameRunner(home, noCommands); !slices.Equal(r.argv, []string{"/opt/run", "--flag"}) {
		t.Fatalf("custom argv = %q", r.argv)
	}
}

func TestUmuOfHeroic(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".config", "heroic", "tools", "runtimes", "umu", "umu_run.py"), "")
	look := func(name string) (string, error) {
		if name == "python3" {
			return "/usr/bin/python3", nil
		}
		return "", errors.New("not found")
	}
	if got := findUmu(home, look); len(got) != 2 || got[0] != "/usr/bin/python3" || !strings.HasSuffix(got[1], "umu_run.py") {
		t.Fatalf("findUmu = %q", got)
	}
}
