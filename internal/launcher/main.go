package launcher

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/TheGreatPepix/awlauncher/internal/region"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	patchJobs      = 2
	patchSourceMiB = 768
)

func installPatches(gameRoot string, patches []patchInfo) error {
	if err := ensureGameClosed(); err != nil {
		return err
	}
	if err := markFXDirty(gameRoot); err != nil {
		return err
	}
	if err := region.Ensure(gameRoot, region.VK); err != nil {
		return err
	}
	build, last, err := currentBuild(gameRoot)
	if err != nil {
		return fmt.Errorf("read game build: %w", err)
	}
	cacheRoot := resolveCacheDir(gameRoot)
	if err := os.MkdirAll(cacheRoot, 0755); err != nil {
		return err
	}
	client := &http.Client{Timeout: 90 * time.Second}
	for _, p := range patches {
		if p.Source != build {
			return errors.New("catalog patch sequence is inconsistent")
		}
		fmt.Printf("\n=== Patch %d -> %d ===\n", p.Source, p.Destination)
		torrent, err := fetch(client, p.TorrentURL, 4<<20)
		if err != nil {
			return err
		}
		if err := verifyHexDigest(torrent, p.TorrentSHA1, "sha1"); err != nil {
			return fmt.Errorf("torrent: %w", err)
		}
		meta, err := parseTorrent(torrent)
		if err != nil {
			return err
		}
		payload := filepath.Join(cacheRoot, fmt.Sprintf("payload-%d-%d", p.Source, p.Destination))
		if err := meta.download(&http.Client{Timeout: 2 * time.Hour}, payload, patchJobs); err != nil {
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
		fmt.Printf("Files built and verified: %d\n", len(names))
		backup, err := os.MkdirTemp(cacheRoot, fmt.Sprintf("backup-%d-%d-", p.Source, p.Destination))
		if err != nil {
			return err
		}
		if err := installPatch(gameRoot, stage, backup, names, last, p, manifest); err != nil {
			return err
		}
		fmt.Printf("Patch %d installed. Backups: %s\n", p.Destination, backup)
		build, last, err = currentBuild(gameRoot)
		if err != nil {
			return err
		}
		if build != p.Destination {
			return errors.New("installed build was not updated")
		}
	}
	cleanupCache(cacheRoot, build)
	return markFXDirty(gameRoot)
}

func resolveCacheDir(gameRoot string) string {
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

func ensureGameClosed() error {
	name, err := runningProcess("ArmoredWarfare.exe")
	if err != nil {
		return err
	}
	if name != "" {
		return errors.New("close the game first")
	}
	return nil
}

func Run() int {
	run := runInteractive
	if args := os.Args[1:]; len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "play":
			run = func() error { return runPlay(strings.Join(args[1:], " ")) }
		case "version", "--version", "-v":
			fmt.Println("AWLauncher", Version)
			return 0
		case "help", "--help", "-h", "/?":
			printUsage()
			return 0
		default:
			fmt.Fprintf(os.Stderr, "Unknown command %q.\n\n", args[0])
			printUsage()
			return 2
		}
	}
	bringLauncherForward = focusConsole
	var err error
	acquired, lockErr := acquireInstance()
	if lockErr != nil {
		err = fmt.Errorf("cannot lock the launcher instance: %w", lockErr)
	} else if acquired {
		err = run()
	} else {
		if activateRunningGUI() {
			fmt.Println("AWLauncher is already running; its window is brought forward.")
		}
		err = errors.New(instanceBusyMessage())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", strings.TrimSpace(err.Error()))
	}
	if ownsConsole() {
		if err != nil {
			fmt.Print("\nPress Enter to close this window...")
			_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		} else {
			time.Sleep(3 * time.Second)
		}
	}
	if err != nil {
		return 1
	}
	return 0
}
