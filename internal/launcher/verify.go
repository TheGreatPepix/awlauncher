package launcher

import (
	"compress/gzip"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/TheGreatPepix/awlauncher/internal/fileutil"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type clientInventory struct {
	Build int             `json:"build"`
	Files []inventoryFile `json:"files"`
}

type inventoryFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func inventoryPath(root string) string {
	return filepath.Join(root, "-gup-", "awlauncher", "inventory.json")
}

func loadClientInventory(root string, build int) (clientInventory, error) {
	data, err := os.ReadFile(inventoryPath(root))
	if err == nil {
		var cached clientInventory
		if json.Unmarshal(data, &cached) == nil && cached.Build == build && len(cached.Files) > 0 {
			return cached, nil
		}
	}
	local := filepath.Join(root, "-gup-", fmt.Sprintf("armoredwarfare_hddistrib%d", build), "manifest.xml.gz")
	if inv, err := inventoryFromManifest(local, build); err == nil {
		return inv, saveClientInventory(root, inv)
	}
	client := &http.Client{Timeout: 90 * time.Second}
	distrib, err := latestDistrib(client)
	if err != nil {
		return clientInventory{}, fmt.Errorf("get client file list: %w", err)
	}
	if distrib.Destination != build {
		return clientInventory{}, fmt.Errorf("no file list for installed build %d; update the game first", build)
	}
	data, err = fetch(client, distrib.TorrentURL, 16<<20)
	if err != nil {
		return clientInventory{}, fmt.Errorf("get client file list: %w", err)
	}
	if err := verifyHexDigest(data, distrib.TorrentSHA1, "sha1"); err != nil {
		return clientInventory{}, fmt.Errorf("client file list: %w", err)
	}
	meta, err := parseTorrent(data)
	if err != nil {
		return clientInventory{}, err
	}
	inv := clientInventory{Build: build}
	for _, f := range meta.files {
		if !f.padding && !strings.EqualFold(f.name, "manifest.xml.gz") && !strings.EqualFold(f.name, "app.7z.001") {
			inv.Files = append(inv.Files, inventoryFile{Name: f.name, Size: f.size})
		}
	}
	if len(inv.Files) == 0 {
		return clientInventory{}, errors.New("empty client file list")
	}
	return inv, saveClientInventory(root, inv)
}

func inventoryFromManifest(path string, build int) (clientInventory, error) {
	f, err := os.Open(path)
	if err != nil {
		return clientInventory{}, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return clientInventory{}, err
	}
	defer gz.Close()
	data, err := io.ReadAll(io.LimitReader(gz, 16<<20+1))
	if err != nil || len(data) > 16<<20 {
		return clientInventory{}, errors.New("invalid local client manifest")
	}
	var manifest patchManifest
	if err := xml.Unmarshal(data, &manifest); err != nil {
		return clientInventory{}, err
	}
	if manifest.Name != "armoredwarfare_hd" || manifest.Build != build || len(manifest.NonCompressed.Files) == 0 {
		return clientInventory{}, errors.New("local client manifest does not match build")
	}
	inv := clientInventory{Build: build}
	for _, f := range manifest.NonCompressed.Files {
		inv.Files = append(inv.Files, inventoryFile{Name: f.Name, Size: f.Size})
	}
	return inv, nil
}

func saveClientInventory(root string, inv clientInventory) error {
	data, err := json.Marshal(inv)
	if err != nil {
		return err
	}
	path := inventoryPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return fileutil.WriteAtomic(path, data)
}

func damagedClientFiles(root string, inv clientInventory) ([]inventoryFile, error) {
	if len(inv.Files) == 0 {
		return nil, errors.New("empty client file list")
	}
	var bad []inventoryFile
	for _, f := range inv.Files {
		path, err := safeGamePath(root, f.Name)
		if err != nil || f.Size < 0 {
			return nil, fmt.Errorf("invalid client file entry %q", f.Name)
		}
		st, err := os.Stat(path)
		wrongSize := !strings.EqualFold(filepath.Clean(f.Name), "user.cfg") && err == nil && st.Size() != f.Size
		if err != nil || !st.Mode().IsRegular() || wrongSize {
			bad = append(bad, f)
		}
	}
	return bad, nil
}

func checkClientFiles(root string, inv clientInventory) (int, string, error) {
	bad, err := damagedClientFiles(root, inv)
	if err != nil {
		return 0, "", err
	}
	if len(bad) == 0 {
		return 0, "", nil
	}
	return len(bad), bad[0].Name, nil
}

func verifyClientBeforeLaunch(p prompter, g gameInstall) error {
	if vkNeedsFullVerification(g.Root) {
		if err := verifyVKAfterFXChanges(p, g); err != nil {
			return err
		}
	}
	inv, err := loadClientInventory(g.Root, g.Build)
	if err != nil {
		return fmt.Errorf("cannot check client files: %w", err)
	}
	bad, err := damagedClientFiles(g.Root, inv)
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		var size int64
		for _, f := range bad {
			size += f.Size
		}
		p.sayf("Client is incomplete: %d missing or wrong-size files (e.g. %s), %s to download.\n", len(bad), bad[0].Name, progress.FormatBytes(size))
		if !p.yes("Download and repair these files now?", true) {
			return errQuit
		}
		if err := repairClientFiles(g, bad); err != nil {
			return fmt.Errorf("client repair failed: %w", err)
		}
		if remaining, err := damagedClientFiles(g.Root, inv); err != nil || len(remaining) > 0 {
			if err != nil {
				return err
			}
			return fmt.Errorf("%d client files are still missing or wrong-size", len(remaining))
		}
	}
	p.sayf("Client files checked: %d files present with expected sizes.\n", len(inv.Files))
	return nil
}

func verifyVKAfterFXChanges(p prompter, g gameInstall) error {
	client := &http.Client{Timeout: 90 * time.Second}
	distrib, err := latestDistrib(client)
	if err != nil {
		return err
	}
	if distrib.Destination != g.Build {
		return fmt.Errorf("VK Play client is build %d, but the installed game is build %d; update it first", distrib.Destination, g.Build)
	}
	torrent, err := fetch(client, distrib.TorrentURL, 16<<20)
	if err != nil {
		return err
	}
	if err := verifyHexDigest(torrent, distrib.TorrentSHA1, "sha1"); err != nil {
		return fmt.Errorf("torrent: %w", err)
	}
	meta, err := parseTorrent(torrent)
	if err != nil {
		return err
	}
	manifest, _, err := fetchFullClientManifest(client, meta, distrib)
	if err != nil {
		return err
	}
	state, full := readVKVerification(g.Root, g.Build)
	bad, err := mismatchedVKFilesSelected(g.Root, manifest, state.Files, full)
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		var size int64
		for _, f := range bad {
			size += f.Size
		}
		p.sayf("VK Play client differs from its manifest: %d files (e.g. %s), %s to repair.\n", len(bad), bad[0].Name, progress.FormatBytes(size))
		if !p.yes("Download and repair these files now?", true) {
			return errQuit
		}
		if err := ensureGameClosed(); err != nil {
			return err
		}
		for _, f := range bad {
			path, _ := safeGamePath(g.Root, f.Name)
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		if err := repairListedFiles(&http.Client{Timeout: 2 * time.Hour}, g.Root, meta, manifest, bad); err != nil {
			return err
		}
		if err := markFXDirty(g.Root); err != nil {
			return err
		}
	}
	return clearVKVerification(g.Root)
}

func mismatchedVKFiles(root string, manifest patchManifest) ([]inventoryFile, error) {
	return mismatchedVKFilesSelected(root, manifest, nil, true)
}

func mismatchedVKFilesSelected(root string, manifest patchManifest, names []string, full bool) ([]inventoryFile, error) {
	if len(manifest.NonCompressed.Files) == 0 {
		return nil, errors.New("empty VK Play client manifest")
	}
	files := manifest.NonCompressed.Files
	if !full {
		wanted := make(map[string]bool, len(names))
		for _, name := range names {
			wanted[normalizedClientName(name)] = true
		}
		files = nil
		for _, f := range manifest.NonCompressed.Files {
			key := normalizedClientName(f.Name)
			if wanted[key] {
				files = append(files, f)
				delete(wanted, key)
			}
		}
		if len(wanted) > 0 {
			files = manifest.NonCompressed.Files
			full = true
		}
	}
	if full {
		fmt.Printf("Checking all VK Play files by MD5 after FX ID changed the shared client...\n")
	} else {
		fmt.Printf("Checking %d VK Play files changed by FX ID...\n", len(files))
	}
	progress.Default.Begin("Checking VK Play", progress.UnitFiles, int64(len(files)), 0)
	defer progress.Default.End()
	var bad []inventoryFile
	for _, f := range files {
		path, err := safeGamePath(root, f.Name)
		if err != nil {
			return nil, err
		}
		ok, err := validStagedFile(path, f)
		if err != nil {
			return nil, fmt.Errorf("check %s: %w", f.Name, err)
		}
		if !ok {
			bad = append(bad, inventoryFile{Name: f.Name, Size: f.Size})
		}
		progress.Default.Add(1)
	}
	return bad, nil
}
