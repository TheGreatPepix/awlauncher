package gamefiles

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/download"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/torrent"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

const (
	patchJobs       = 2
	patchSourceMiB  = 768
	prefetchReserve = 4 << 30
)

func InstallPatches(gameRoot string, patches []patchInfo, keepBackups bool) error {
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
	var next *prefetch
	defer func() { next.stop() }()
	for i, p := range patches {
		if p.Source != build {
			return errors.New("catalog patch sequence is inconsistent")
		}
		log.Printf("\n=== Patch %d -> %d ===\n", p.Source, p.Destination)
		meta, ok := next.stop()
		next = nil
		if !ok {
			if meta, err = fetchTorrent(client, p.TorrentURL, p.TorrentSHA1); err != nil {
				return err
			}
		}
		payload := patchDir(cacheRoot, p)
		if err := meta.Download(patchClient(), payload, patchJobs); err != nil {
			return err
		}
		if i+1 < len(patches) {
			next = startPrefetch(client, cacheRoot, patches[i+1])
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
		if keepBackups {
			log.Printf("Patch %d installed. Backups: %s\n", p.Destination, backup)
		} else {
			_ = os.RemoveAll(backup)
			log.Printf("Patch %d installed.\n", p.Destination)
		}
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

func patchDir(cacheRoot string, p patchInfo) string {
	return filepath.Join(cacheRoot, fmt.Sprintf("patch-%d-%d", p.Source, p.Destination))
}

func patchClient() *http.Client {
	return &http.Client{Timeout: 2 * time.Hour, Transport: download.Transport}
}

type prefetch struct {
	meta   torrent.Meta
	cancel context.CancelFunc
	done   chan struct{}
}

func startPrefetch(client *http.Client, cacheRoot string, p patchInfo) *prefetch {
	meta, err := fetchTorrent(client, p.TorrentURL, p.TorrentSHA1)
	if err != nil {
		return nil
	}
	payload := patchDir(cacheRoot, p)
	missing := meta.Missing(payload)
	if missing == 0 {
		return &prefetch{meta: meta}
	}
	if free, err := platform.DiskFree(cacheRoot); err != nil || free < missing+prefetchReserve {
		return &prefetch{meta: meta}
	}
	ctx, cancel := context.WithCancel(context.Background())
	pre := &prefetch{meta: meta, cancel: cancel, done: make(chan struct{})}
	log.Printf("Patch %d -> %d downloads in the background meanwhile (%s).\n", p.Source, p.Destination, progress.FormatBytes(missing))
	go func() {
		defer close(pre.done)
		err := meta.Prefetch(ctx, patchClient(), payload, patchJobs, progress.Default.Quiet())
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("The background download of patch %d -> %d stopped: %v; it goes on when that patch is next.\n", p.Source, p.Destination, err)
		}
	}()
	return pre
}

func (p *prefetch) stop() (torrent.Meta, bool) {
	if p == nil {
		return torrent.Meta{}, false
	}
	if p.cancel != nil {
		p.cancel()
		<-p.done
	}
	return p.meta, true
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
