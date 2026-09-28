package gamefiles

import (
	"compress/gzip"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/fileutil"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/catalog"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
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
	distrib, err := catalog.LatestDistrib(client)
	if err != nil {
		return clientInventory{}, fmt.Errorf("get client file list: %w", err)
	}
	if distrib.Destination != build {
		return clientInventory{}, fmt.Errorf("no file list for installed build %d; update the game first", build)
	}
	meta, err := fetchTorrent(client, distrib.TorrentURL, distrib.TorrentSHA1)
	if err != nil {
		return clientInventory{}, fmt.Errorf("get client file list: %w", err)
	}
	inv := clientInventory{Build: build}
	for _, f := range meta.Files {
		if !f.Padding && !strings.EqualFold(f.Name, "manifest.xml.gz") && !strings.EqualFold(f.Name, "app.7z.001") {
			inv.Files = append(inv.Files, inventoryFile{Name: f.Name, Size: f.Size})
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
	var manifest Manifest
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

func damagedClientFiles(root string, inv clientInventory, allowMods bool) ([]inventoryFile, error) {
	if len(inv.Files) == 0 {
		return nil, errors.New("empty client file list")
	}
	var bad []inventoryFile
	for _, f := range inv.Files {
		path, err := SafePath(root, f.Name)
		if err != nil || f.Size < 0 {
			return nil, fmt.Errorf("invalid client file entry %q", f.Name)
		}
		st, err := os.Stat(path)
		wrongSize := !allowMods && !strings.EqualFold(filepath.Clean(f.Name), "user.cfg") && err == nil && st.Size() != f.Size
		if err != nil || !st.Mode().IsRegular() || wrongSize {
			bad = append(bad, f)
		}
	}
	return bad, nil
}

func checkClientFiles(root string, inv clientInventory) (int, string, error) {
	bad, err := damagedClientFiles(root, inv, false)
	if err != nil {
		return 0, "", err
	}
	if len(bad) == 0 {
		return 0, "", nil
	}
	return len(bad), bad[0].Name, nil
}

func VerifyBeforeLaunch(p Asker, g Install, allowMods bool) error {
	inv, err := loadClientInventory(g.Root, g.Build)
	if err != nil {
		return fmt.Errorf("cannot check client files: %w", err)
	}
	bad, err := damagedClientFiles(g.Root, inv, allowMods)
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		var size int64
		for _, f := range bad {
			size += f.Size
		}
		p.Sayf("Client is incomplete: %d missing or wrong-size files (e.g. %s), %s to download.\n", len(bad), bad[0].Name, progress.FormatBytes(size))
		if !p.Yes("Download and repair these files now?", true) {
			return ErrDeclined
		}
		if err := repairClientFiles(g, bad); err != nil {
			return fmt.Errorf("client repair failed: %w", err)
		}
		if remaining, err := damagedClientFiles(g.Root, inv, allowMods); err != nil || len(remaining) > 0 {
			if err != nil {
				return err
			}
			return fmt.Errorf("%d client files are still missing or wrong-size", len(remaining))
		}
	}
	if allowMods {
		p.Sayf("Client files checked: %d files present; modified files kept (mods enabled).\n", len(inv.Files))
	} else {
		p.Sayf("Client files checked: %d files present with expected sizes.\n", len(inv.Files))
	}
	return nil
}

func VerifyVKFiles(p Asker, g Install) error {
	d, err := fetchVKDistrib(g.Build)
	if err != nil {
		return err
	}
	set, err := d.files(nil)
	if err != nil {
		return err
	}
	log.Printf("Checking all VK Play files by MD5...\n")
	_, err = syncFiles(g.Root, set, syncOptions{
		Jobs: 3,
		Confirm: func(bad []remoteFile, size int64) error {
			p.Sayf("VK Play client differs from its manifest: %d files (e.g. %s), %s to repair.\n", len(bad), bad[0].Path, progress.FormatBytes(size))
			if !p.Yes("Download and repair these files now?", true) {
				return ErrDeclined
			}
			return EnsureGameClosed()
		},
	})
	return err
}
