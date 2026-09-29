package gamefiles

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/download"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
)

const (
	patchJobs      = 2
	patchSourceMiB = 768
)

func InstallPatches(gameRoot string, patches []patchInfo) error {
	if err := EnsureGameClosed(); err != nil {
		return err
	}
	build, last, err := CurrentBuild(gameRoot)
	if err != nil {
		return fmt.Errorf("read game build: %w", err)
	}
	cacheRoot := CacheDir(gameRoot)
	if err := os.MkdirAll(cacheRoot, 0755); err != nil {
		return err
	}
	client := &http.Client{Timeout: 90 * time.Second}
	for _, p := range patches {
		if p.Source != build {
			return errors.New("catalog patch sequence is inconsistent")
		}
		log.Printf("\n=== Patch %d -> %d ===\n", p.Source, p.Destination)
		meta, err := fetchTorrent(client, p.TorrentURL, p.TorrentSHA1)
		if err != nil {
			return err
		}
		payload := filepath.Join(cacheRoot, fmt.Sprintf("payload-%d-%d", p.Source, p.Destination))
		if err := meta.Download(&http.Client{Timeout: 2 * time.Hour, Transport: download.Transport}, payload, patchJobs); err != nil {
			return err
		}
		manifest, err := loadManifest(payload, p)
		if err != nil {
			return err
		}
		stage := filepath.Join(cacheRoot, fmt.Sprintf("stage-%d-%d", p.Source, p.Destination))
		if err := os.MkdirAll(stage, 0755); err != nil {
			return err
		}
		names, err := stagePatch(gameRoot, payload, stage, manifest, patchJobs, patchSourceMiB)
		if err != nil {
			return err
		}
		log.Printf("Files built and verified: %d\n", len(names))
		backup, err := os.MkdirTemp(cacheRoot, fmt.Sprintf("backup-%d-%d-", p.Source, p.Destination))
		if err != nil {
			return err
		}
		if err := installPatch(gameRoot, stage, backup, names, last, p, manifest); err != nil {
			return err
		}
		log.Printf("Patch %d installed. Backups: %s\n", p.Destination, backup)
		build, last, err = CurrentBuild(gameRoot)
		if err != nil {
			return err
		}
		if build != p.Destination {
			return errors.New("installed build was not updated")
		}
	}
	CleanupCache(cacheRoot, build)
	return nil
}

func CacheDir(gameRoot string) string {
	dir := filepath.Join(gameRoot, "-gup-", "awlauncher-cache")
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	old := filepath.Join(gameRoot, "-gup-", "awpatcher-cache")
	if st, err := os.Stat(old); err == nil && st.IsDir() {
		return old
	}
	return dir
}

func EnsureGameClosed() error {
	name, err := platform.RunningProcess(platform.GameExe)
	if err != nil {
		return err
	}
	if name != "" {
		return errors.New("close the game first")
	}
	return nil
}
