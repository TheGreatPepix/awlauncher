package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFitCase(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Bin64", "Shaders"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Bin64", "CrySystem.dll"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"Bin64/CrySystem.dll":    "Bin64/CrySystem.dll",
		"bin64/crysystem.DLL":    "Bin64/CrySystem.dll",
		"BIN64/shaders/new.pak":  "Bin64/Shaders/new.pak",
		"bin64/Missing/file.txt": "Bin64/Missing/file.txt",
		"Data/levels/level.pak":  "Data/levels/level.pak",
		"bin64/shaders":          "Bin64/Shaders",
	}
	for rel, want := range cases {
		got := FitCase(root, filepath.FromSlash(rel))
		if got != filepath.Join(root, filepath.FromSlash(want)) {
			t.Errorf("FitCase(%q) = %q, want %q", rel, got, want)
		}
	}
}

func TestProgramName(t *testing.T) {
	cases := map[string]string{
		"Z:\\home\\deck\\Games\\Armored Warfare\\bin64\\ArmoredWarfare.exe\x00-arg\x00": "ArmoredWarfare.exe",
		"C:\\windows\\system32\\steam.exe\x00Z:\\games\\ArmoredWarfare.exe\x00":         "steam.exe",
		"/usr/bin/python3\x00/proton\x00waitforexitandrun\x00ArmoredWarfare.exe\x00":    "python3",
		"": "",
	}
	for cmdline, want := range cases {
		if got := programName([]byte(cmdline)); got != want {
			t.Errorf("programName(%q) = %q, want %q", cmdline, got, want)
		}
	}
}

func TestLauncherDirFollowsXDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/data")
	if dir, err := DataDir(); err != nil || dir != "/data/awlauncher" {
		t.Fatalf("DataDir = %q, %v", dir, err)
	}
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "/home/deck")
	if dir, err := DataDir(); err != nil || dir != "/home/deck/.local/share/awlauncher" {
		t.Fatalf("DataDir = %q, %v", dir, err)
	}
}
