package launcher

import (
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/TheGreatPepix/awlauncher/internal/fileutil"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
	"github.com/TheGreatPepix/awlauncher/internal/region"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const gib = 1 << 30

var errInstallCancelled = errors.New("install cancelled")

func installGame(p prompter, dir, fxRoot string) error {
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if err := markFXDirty(root); err != nil {
		return err
	}
	if isGameDir(root) {
		if err := region.Ensure(root, region.VK); err != nil {
			return err
		}
	}
	client := &http.Client{Timeout: 90 * time.Second}
	p.say("Looking up the latest full client...")
	distrib, err := latestDistrib(client)
	if err != nil {
		return err
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

	var total, have int64
	for _, f := range meta.files {
		if f.padding {
			continue
		}
		total += f.size
		if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(f.name))); err == nil && st.Size() == f.size {
			have += f.size
		}
	}
	need := total - have
	p.sayf("Build %d, %.1f GiB.\n", distrib.Destination, float64(total)/gib)
	if have > 0 {
		p.sayf("Resuming: %.1f GiB already in place, %.1f GiB left.\n", float64(have)/gib, float64(need)/gib)
	} else if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 {
		p.sayf("%s is not empty. Files with the same names as the game's will be overwritten.\n", root)
		if !p.yes("Install there anyway?", false) {
			return errInstallCancelled
		}
	}
	if !p.yes(fmt.Sprintf("Install Armored Warfare into %s?", root), true) {
		return errInstallCancelled
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	if fxRoot != "" && !strings.EqualFold(filepath.Clean(fxRoot), filepath.Clean(root)) {
		if state, ok := readBranchState(fxRoot); ok && state.Branch == fxDefaultBranch {
			p.say("Checking which FX ID client files can be reused for VK Play...")
			manifest, compressed, err := fetchFullClientManifest(client, meta, distrib)
			if err != nil {
				return err
			}
			if err := fileutil.WriteAtomic(filepath.Join(root, "manifest.xml.gz"), compressed); err != nil {
				return err
			}
			linked, copied, err := seedVKFromFX(root, fxRoot, manifest)
			if err != nil {
				return err
			}
			p.sayf("Reused %s as hard links and %s as copies from FX ID.\n", progress.FormatBytes(linked), progress.FormatBytes(copied))
		}
	}
	need = 0
	for _, f := range meta.files {
		if f.padding {
			continue
		}
		if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(f.name))); err != nil || st.Size() != f.size {
			need += f.size
		}
	}
	if free, err := diskFree(root); err == nil {
		p.sayf("Free space: %.1f GiB; remaining download: %.1f GiB.\n", float64(free)/gib, float64(need)/gib)
		if free < need+gib {
			return fmt.Errorf("not enough free space: %.1f GiB needed", float64(need+gib)/gib)
		}
	}

	download := &http.Client{Timeout: 2 * time.Hour}
	for attempt := 1; ; attempt++ {
		if err := meta.fetchFiles(download, root, 4); err != nil {
			return fmt.Errorf("%w (run the launcher again to resume)", err)
		}
		bad, err := meta.badFiles(root)
		if err != nil {
			return err
		}
		if len(bad) == 0 {
			break
		}
		if attempt == 3 {
			return fmt.Errorf("%d files are still corrupt after downloading them again, e.g. %s", len(bad), bad[0])
		}
		p.sayf("%d corrupt files, downloading them again.\n", len(bad))
		for _, name := range bad {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(name))); err != nil {
				return err
			}
		}
	}

	manifest, err := loadManifest(root, distrib)
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	inManifest := map[string]bool{}
	for _, f := range manifest.NonCompressed.Files {
		path, err := safeGamePath(root, f.Name)
		if err != nil {
			return err
		}
		if st, err := os.Stat(path); err != nil || st.Size() != f.Size {
			return fmt.Errorf("missing or wrong size: %s", f.Name)
		}
		inManifest[strings.ToLower(filepath.Clean(f.Name))] = true
	}
	for _, folder := range manifest.NonCompressed.Folders {
		path, err := safeGamePath(root, folder.Name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}

	gup := filepath.Join(root, "-gup-")
	distribDir := filepath.Join(gup, fmt.Sprintf("armoredwarfare_hddistrib%d", distrib.Destination))
	if err := os.MkdirAll(distribDir, 0755); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(root, "manifest.xml.gz"), filepath.Join(distribDir, "manifest.xml.gz")); err != nil {
		return err
	}
	for _, f := range meta.files {
		name := filepath.FromSlash(f.name)
		if f.padding || strings.EqualFold(name, "manifest.xml.gz") || inManifest[strings.ToLower(name)] {
			continue
		}
		if err := os.Remove(filepath.Join(root, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	if err := writeInstalledLastXML(gup, distrib, manifest); err != nil {
		return err
	}
	if err := markFXDirty(root); err != nil {
		return err
	}
	p.sayf("Armored Warfare %d is installed in %s.\n", distrib.Destination, root)
	offerRedist(p, root)
	return nil
}

func writeInstalledLastXML(gup string, distrib patchInfo, manifest patchManifest) error {
	now := strconv.FormatInt(time.Now().Unix(), 10)
	last := xmlElement{XMLName: xml.Name{Local: "Manifest"}}
	for _, a := range [][2]string{
		{"AutoUpdate", "2"},
		{"Build", strconv.Itoa(manifest.Build)},
		{"NeedVerification", "0"},
		{"Pure", "1"},
		{"VersionUnixTime", strconv.FormatInt(distrib.ModifiedUnix, 10)},
		{"InstallDate", now},
		{"InstalledSize", strconv.FormatInt(distrib.InstalledSize, 10)},
		{"TimeStamp", now},
	} {
		last.Attrs = setAttr(last.Attrs, a[0], a[1])
	}
	misc := mergeMisc(xmlElement{}, manifest.Misc)
	misc.Attrs = setAttrFold(misc.Attrs, "GAMEID", "0."+gameProjectID)
	last.Children = append(last.Children, misc)
	if manifest.RunCheck.XMLName.Local != "" {
		last.Children = append(last.Children, manifest.RunCheck)
	}
	data, err := xml.MarshalIndent(last, "", "\t")
	if err != nil {
		return err
	}
	data = append([]byte(xml.Header), data...)
	path := filepath.Join(gup, "last.xml")
	if err := os.WriteFile(path+".tmp", data, 0644); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

func offerRedist(p prompter, root string) {
	redist := filepath.Join(root, "vc_redist.x64.exe")
	if _, err := os.Stat(redist); err != nil {
		return
	}
	if !p.yes("Install Microsoft Visual C++ Redistributable (the game needs it)?", true) {
		p.sayf("You can install it later: %s\n", redist)
		return
	}
	err := runRedist(redist)
	var exit *exec.ExitError
	switch {
	case err == nil:
		p.say("Visual C++ Redistributable installed.")
	case errors.As(err, &exit) && exit.ExitCode() == 1638:
		p.say("A newer Visual C++ Redistributable is already installed.")
	case errors.As(err, &exit) && exit.ExitCode() == 3010:
		p.say("Visual C++ Redistributable installed; restart Windows before playing.")
	default:
		p.sayf("Visual C++ Redistributable was not installed (%v). You can run it later: %s\n", err, redist)
	}
}
