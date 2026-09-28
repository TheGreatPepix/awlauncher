package gamefiles

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/fileutil"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/vkauth"
)

const gib = 1 << 30

var ErrInstallCancelled = errors.New("install cancelled")

func InstallVK(p Asker, dir string) error {
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	p.Say("Looking up the latest full client...")
	d, err := fetchVKDistrib(0)
	if err != nil {
		return err
	}
	set, err := d.files(nil)
	if err != nil {
		return err
	}
	var total, have int64
	for _, f := range set.Files {
		total += f.Size
		if path, err := SafePath(root, f.Path); err == nil {
			if st, err := os.Stat(path); err == nil && st.Size() == f.Size {
				have += f.Size
			}
		}
	}
	need := total - have
	p.Sayf("Build %d, %.1f GiB.\n", d.info.Destination, float64(total)/gib)
	if have > 0 {
		p.Sayf("Resuming: %.1f GiB already in place, %.1f GiB left.\n", float64(have)/gib, float64(need)/gib)
	} else if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 {
		p.Sayf("%s is not empty. Files with the same names as the game's will be overwritten.\n", root)
		if !p.Yes("Install there anyway?", false) {
			return ErrInstallCancelled
		}
	}
	if !p.Yes(fmt.Sprintf("Install Armored Warfare into %s?", root), true) {
		return ErrInstallCancelled
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	if free, err := platform.DiskFree(root); err == nil {
		p.Sayf("Free space: %.1f GiB; remaining download: %.1f GiB.\n", float64(free)/gib, float64(need)/gib)
	}
	if _, err := syncFiles(root, set, syncOptions{Jobs: 4, Reserve: gib}); err != nil {
		return err
	}
	for _, folder := range d.manifest.NonCompressed.Folders {
		path, err := SafePath(root, folder.Name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	gup := filepath.Join(root, "-gup-")
	distribDir := filepath.Join(gup, fmt.Sprintf("armoredwarfare_hddistrib%d", d.info.Destination))
	if err := os.MkdirAll(distribDir, 0755); err != nil {
		return err
	}
	if err := fileutil.WriteAtomic(filepath.Join(distribDir, "manifest.xml.gz"), d.compressed); err != nil {
		return err
	}
	if err := writeInstalledLastXML(gup, d.info, d.manifest); err != nil {
		return err
	}
	p.Sayf("Armored Warfare %d is installed in %s.\n", d.info.Destination, root)
	offerRedist(p, root)
	return nil
}

func writeInstalledLastXML(gup string, distrib patchInfo, manifest Manifest) error {
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
	misc.Attrs = setAttrFold(misc.Attrs, "GAMEID", "0."+vkauth.GameProjectID)
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

func offerRedist(p Asker, root string) {
	redist := filepath.Join(root, "vc_redist.x64.exe")
	if _, err := os.Stat(redist); err != nil {
		return
	}
	if !p.Yes("Install Microsoft Visual C++ Redistributable (the game needs it)?", true) {
		p.Sayf("You can install it later: %s\n", redist)
		return
	}
	err := platform.RunRedist(redist)
	var exit *exec.ExitError
	switch {
	case err == nil:
		p.Say("Visual C++ Redistributable installed.")
	case errors.As(err, &exit) && exit.ExitCode() == 1638:
		p.Say("A newer Visual C++ Redistributable is already installed.")
	case errors.As(err, &exit) && exit.ExitCode() == 3010:
		p.Say("Visual C++ Redistributable installed; restart Windows before playing.")
	default:
		p.Sayf("Visual C++ Redistributable was not installed (%v). You can run it later: %s\n", err, redist)
	}
}
