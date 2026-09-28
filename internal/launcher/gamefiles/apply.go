package gamefiles

import (
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/catalog"
	"github.com/TheGreatPepix/awlauncher/internal/progress"

	"github.com/bodgit/sevenzip"
	"github.com/go-deltasync/vcdiff"
)

func loadManifest(payload string, patch patchInfo) (Manifest, error) {
	compressed, err := os.ReadFile(filepath.Join(payload, "manifest.xml.gz"))
	if err != nil {
		return Manifest{}, err
	}
	return parsePatchManifest(compressed, patch)
}

func parsePatchManifest(compressed []byte, patch patchInfo) (Manifest, error) {
	var manifest Manifest
	if err := catalog.VerifyHexDigest(compressed, patch.ManifestSHA, "sha256"); err != nil {
		return manifest, err
	}
	gz, err := gzip.NewReader(strings.NewReader(string(compressed)))
	if err != nil {
		return manifest, err
	}
	defer gz.Close()
	data, err := io.ReadAll(io.LimitReader(gz, 16<<20))
	if err != nil {
		return manifest, err
	}
	if len(data) == 16<<20 {
		return manifest, errors.New("manifest too large")
	}
	if err = xml.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	if manifest.Name != "armoredwarfare_hd" || manifest.Build != patch.Destination || manifest.FromBuild != patch.Source {
		return manifest, errors.New("manifest build or game mismatch")
	}
	return manifest, nil
}

func archivedFile(archive *sevenzip.ReadCloser, name string) (*sevenzip.File, error) {
	want := strings.ToLower(strings.ReplaceAll(name, `\`, "/"))
	for _, f := range archive.File {
		if strings.ToLower(strings.ReplaceAll(f.Name, `\`, "/")) == want {
			return f, nil
		}
	}
	return nil, fmt.Errorf("archive member %q missing", name)
}

func verifiedOutput(dst string, item ManifestFile, produce func(io.Writer) error) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	h := md5.New()
	task := progress.Default.Start(item.Name, item.Size)
	defer task.Done()
	w := io.MultiWriter(f, h, task)
	err = produce(w)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(dst)
		return err
	}
	info, err := os.Stat(dst)
	if err != nil {
		return err
	}
	if info.Size() != item.Size || !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), item.MD5) {
		os.Remove(dst)
		return fmt.Errorf("size or MD5 mismatch for %s", item.Name)
	}
	if item.Modified > 0 {
		m := time.Unix(item.Modified, 0)
		_ = os.Chtimes(dst, m, m)
	}
	return nil
}

func stagePatch(game, payload, stage string, manifest Manifest, jobs, maxSourceMiB int) ([]string, error) {
	app, err := sevenzip.OpenReader(filepath.Join(payload, "app.7z.001"))
	if err != nil {
		return nil, err
	}
	defer app.Close()
	delta, err := sevenzip.OpenReader(filepath.Join(payload, "patch.7z.001"))
	if err != nil {
		return nil, err
	}
	defer delta.Close()
	var names []string
	seen := make(map[string]bool)
	var totalSize int64
	for _, item := range manifest.Files.Files {
		totalSize += item.Size
	}
	for _, item := range manifest.PatchFiles.Files {
		totalSize += item.Size
	}
	progress.Default.Begin("Building", progress.UnitBytes, totalSize, 0)
	defer progress.Default.End()
	for _, item := range manifest.Files.Files {
		if err := stageName(stage, item.Name, seen, &names); err != nil {
			return nil, err
		}
		member, err := archivedFile(app, item.Name)
		if err != nil {
			return nil, err
		}
		out, err := SafePath(stage, item.Name)
		if err != nil {
			return nil, err
		}
		if valid, err := fileMatches(out, item.Size, item.MD5, md5.New, nil); err != nil {
			return nil, err
		} else if valid {
			progress.Default.Add(item.Size)
			continue
		}
		err = verifiedOutput(out, item, func(w io.Writer) error {
			r, err := member.Open()
			if err != nil {
				return err
			}
			defer r.Close()
			_, err = io.Copy(w, r)
			return err
		})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", item.Name, err)
		}
	}
	if jobs < 1 {
		jobs = 1
	}
	if jobs > 8 {
		jobs = 8
	}
	if maxSourceMiB < 64 {
		maxSourceMiB = 64
	}
	budget := (maxSourceMiB + 63) / 64
	mem := make(chan struct{}, budget)
	slots := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	var firstErr error
	var once sync.Once
	var failed atomic.Bool
	recordErr := func(err error) { once.Do(func() { firstErr = err; failed.Store(true) }) }
	for i, item := range manifest.PatchFiles.Files {
		if err := stageName(stage, item.Name, seen, &names); err != nil {
			recordErr(err)
			break
		}
		member, err := archivedFile(delta, fmt.Sprint(i))
		if err != nil {
			recordErr(err)
			break
		}
		sourcePath, err := SafePath(game, item.Name)
		if err != nil {
			recordErr(err)
			break
		}
		out, err := SafePath(stage, item.Name)
		if err != nil {
			recordErr(err)
			break
		}
		if valid, err := fileMatches(out, item.Size, item.MD5, md5.New, nil); err != nil {
			recordErr(err)
			break
		} else if valid {
			progress.Default.Add(item.Size)
			continue
		}
		stat, err := os.Stat(sourcePath)
		if err != nil {
			recordErr(err)
			break
		}
		weight := int((stat.Size() + (64 << 20) - 1) / (64 << 20))
		if weight < 1 {
			weight = 1
		}
		if weight > budget {
			weight = budget
		}
		for n := 0; n < weight; n++ {
			mem <- struct{}{}
		}
		slots <- struct{}{}
		if failed.Load() {
			for n := 0; n < weight; n++ {
				<-mem
			}
			<-slots
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				for n := 0; n < weight; n++ {
					<-mem
				}
				<-slots
			}()
			source, err := os.ReadFile(sourcePath)
			if err == nil {
				err = verifiedOutput(out, item, func(w io.Writer) error {
					r, e := member.Open()
					if e != nil {
						return e
					}
					defer r.Close()
					_, e = vcdiff.Decode(source, r, w)
					return e
				})
			}
			if err != nil {
				recordErr(fmt.Errorf("%s: %w", item.Name, err))
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return names, nil
}

func stageName(root, name string, seen map[string]bool, names *[]string) error {
	if _, err := SafePath(root, name); err != nil {
		return err
	}
	key := strings.ToLower(strings.ReplaceAll(name, `\`, "/"))
	if seen[key] {
		return fmt.Errorf("duplicate manifest file %s", name)
	}
	seen[key] = true
	*names = append(*names, name)
	return nil
}

func copyFile(source, dst string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(dst)
		return err
	}
	if stat, err := in.Stat(); err == nil {
		_ = os.Chtimes(dst, stat.ModTime(), stat.ModTime())
	}
	return nil
}

type replacement struct {
	target, backup string
	existed        bool
}

func installPatch(game, stage, backup string, names []string, last []byte, patch patchInfo, manifest Manifest) error {
	updated, err := newLastXML(last, patch, manifest)
	if err != nil {
		return err
	}
	lastPath := filepath.Join(game, "-gup-", "last.xml")
	names = append(append([]string(nil), names...), filepath.Join("-gup-", "last.xml"))
	fastMove := strings.EqualFold(filepath.VolumeName(game), filepath.VolumeName(stage)) && strings.EqualFold(filepath.VolumeName(game), filepath.VolumeName(backup))
	progress.Default.Begin("Installing", progress.UnitFiles, int64(len(names)), 0)
	progress.Default.NoPause()
	defer progress.Default.End()
	var done []replacement
	rollback := func() {
		for i := len(done) - 1; i >= 0; i-- {
			r := done[i]
			_ = os.Remove(r.target)
			if r.existed {
				var err error
				if fastMove {
					err = os.Rename(r.backup, r.target)
				} else {
					err = copyFile(r.backup, r.target)
				}
				if err != nil {
					progress.Default.Log("ROLLBACK ERROR %s: %v", r.target, err)
				}
			}
		}
	}
	for _, name := range names {
		target, err := SafePath(game, name)
		if err != nil {
			rollback()
			return err
		}
		bak, err := SafePath(backup, name)
		if err != nil {
			rollback()
			return err
		}
		_, statErr := os.Stat(target)
		existed := statErr == nil
		if statErr != nil && !os.IsNotExist(statErr) {
			rollback()
			return statErr
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			rollback()
			return err
		}
		if existed {
			if err := os.MkdirAll(filepath.Dir(bak), 0755); err != nil {
				rollback()
				return err
			}
			var backupErr error
			if fastMove {
				backupErr = os.Rename(target, bak)
			} else {
				backupErr = copyFile(target, bak)
			}
			if backupErr != nil {
				rollback()
				return backupErr
			}
		}
		if fastMove {
			done = append(done, replacement{target: target, backup: bak, existed: existed})
		}
		task := progress.Default.Start(name, 0)
		if name == filepath.Join("-gup-", "last.xml") {
			tmp := lastPath + ".awlauncher.tmp"
			if err := os.WriteFile(tmp, updated, 0644); err != nil {
				rollback()
				return err
			}
			err = os.Rename(tmp, target)
		} else {
			src, _ := SafePath(stage, name)
			if fastMove {
				err = os.Rename(src, target)
			} else {
				tmp := target + ".awlauncher.tmp"
				err = copyFile(src, tmp)
				if err == nil {
					err = os.Rename(tmp, target)
				}
			}
		}
		task.Done()
		progress.Default.Add(1)
		if err != nil {
			rollback()
			return fmt.Errorf("install %s: %w", name, err)
		}
		if !fastMove {
			done = append(done, replacement{target: target, backup: bak, existed: existed})
		}
	}
	return nil
}
