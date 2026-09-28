package gamefiles

import (
	"fmt"
	"path/filepath"
)

type Install struct {
	Root   string
	Build  int
	Launch LaunchConfig
	Exe    string
}

func Open(dir string) (Install, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return Install{}, err
	}
	build, last, err := CurrentBuild(root)
	if err != nil {
		return Install{}, fmt.Errorf("read game build: %w", err)
	}
	lc, err := readLaunchConfig(last)
	if err != nil {
		return Install{}, err
	}
	exe, err := SafePath(root, lc.Exe)
	if err != nil {
		return Install{}, err
	}
	return Install{Root: root, Build: build, Launch: lc, Exe: exe}, nil
}
