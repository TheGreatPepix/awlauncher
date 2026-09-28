package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/fileutil"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
)

type uiPrefs struct {
	Theme string `json:"theme"`
	Hue   int    `json:"hue"`
	Lang  string `json:"lang"`
}

func prefsPath() (string, error) {
	dir, err := platform.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ui.json"), nil
}

func loadPrefs() uiPrefs {
	p := uiPrefs{Theme: "system", Hue: 300, Lang: "auto"}
	if path, err := prefsPath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, &p)
		}
	}
	return p
}

func savePrefs(p uiPrefs) error {
	path, err := prefsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.WriteAtomic(path, data)
}

func parseHexColor(s string) ([3]uint8, bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return [3]uint8{}, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return [3]uint8{}, false
	}
	return [3]uint8{uint8(v >> 16), uint8(v >> 8), uint8(v)}, true
}
