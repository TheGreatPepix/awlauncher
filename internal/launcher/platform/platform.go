package platform

import (
	"os"
	"path/filepath"
)

const GameExe = "ArmoredWarfare.exe"

func RunningProcess(names ...string) (string, error) {
	for _, name := range names {
		ids, err := processIDs(name)
		if err != nil {
			return "", err
		}
		if len(ids) > 0 {
			return name, nil
		}
	}
	return "", nil
}

func GameRunning() bool {
	ids, err := processIDs(GameExe)
	return err == nil && len(ids) > 0
}

func existingDir(dir string) string {
	for {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}
