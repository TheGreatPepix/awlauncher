package launcher

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "awlauncher-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("LOCALAPPDATA", dir)
	os.Setenv("XDG_DATA_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
