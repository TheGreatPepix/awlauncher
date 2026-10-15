package gamefiles

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/download"
	"github.com/TheGreatPepix/awlauncher/internal/fileutil"
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
	if err := download.VerifyHexDigest(compressed, patch.ManifestSHA, "sha256"); err != nil {
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

type archiveMember struct {
	file  *sevenzip.File
	index int
}

// archiveIndex maps member names to their position in the archive. Reading
// members in that order lets each solid block be decompressed only once.
func archiveIndex(archive *sevenzip.ReadCloser) map[string]archiveMember {
	members := make(map[string]archiveMember, len(archive.File))
	for i, f := range archive.File {
		members[memberKey(f.Name)] = archiveMember{f, i}
	}
	return members
}

func memberKey(name string) string { return strings.ToLower(strings.ReplaceAll(name, `\`, "/")) }

func archivedFile(members map[string]archiveMember, name string) (archiveMember, error) {
	if m, ok := members[memberKey(name)]; ok {
		return m, nil
	}
	return archiveMember{}, fmt.Errorf("archive member %q missing", name)
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
	err = pipeAhead(io.MultiWriter(f, h, task), produce)
	if err == nil {
		err = f.Sync()
	}
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

type stageJob struct {
	item   ManifestFile
	member archiveMember
	out    string
	source string
}

// memBudget limits how much memory the delta workers hold at once.
type memBudget struct {
	mu    sync.Mutex
	cond  *sync.Cond
	free  int64
	total int64
}

func newMemBudget(total int64) *memBudget {
	b := &memBudget{free: total, total: total}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (b *memBudget) acquire(n int64) int64 {
	n = min(max(n, 1), b.total)
	b.mu.Lock()
	for b.free < n {
		b.cond.Wait()
	}
	b.free -= n
	b.mu.Unlock()
	return n
}

func (b *memBudget) release(n int64) {
	b.mu.Lock()
	b.free += n
	b.mu.Unlock()
	b.cond.Broadcast()
}

// deltaMemory estimates the peak memory of one vcdiff job: the whole source,
// the compressed delta, and the decoder's copy of the target, which grows by
// append and so can briefly need twice its size.
func deltaMemory(sourceSize, deltaSize, targetSize int64) int64 {
	return sourceSize + deltaSize + 2*targetSize
}

func stagePatch(game, payload, stage string, manifest Manifest, jobs int, budget int64) ([]string, error) {
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
	appMembers, deltaMembers := archiveIndex(app), archiveIndex(delta)

	var names []string
	seen := make(map[string]bool)
	var totalSize int64
	var appJobs, deltaJobs []stageJob
	for _, item := range manifest.Files.Files {
		if err := stageName(stage, item.Name, seen, &names); err != nil {
			return nil, err
		}
		member, err := archivedFile(appMembers, item.Name)
		if err != nil {
			return nil, err
		}
		out, err := SafePath(stage, item.Name)
		if err != nil {
			return nil, err
		}
		appJobs = append(appJobs, stageJob{item: item, member: member, out: out})
		totalSize += item.Size
	}
	for i, item := range manifest.PatchFiles.Files {
		if err := stageName(stage, item.Name, seen, &names); err != nil {
			return nil, err
		}
		member, err := archivedFile(deltaMembers, fmt.Sprint(i))
		if err != nil {
			return nil, err
		}
		source, err := SafePath(game, item.Name)
		if err != nil {
			return nil, err
		}
		out, err := SafePath(stage, item.Name)
		if err != nil {
			return nil, err
		}
		deltaJobs = append(deltaJobs, stageJob{item: item, member: member, out: out, source: source})
		totalSize += item.Size
	}
	byArchiveOrder := func(list []stageJob) {
		sort.Slice(list, func(i, j int) bool { return list[i].member.index < list[j].member.index })
	}
	byArchiveOrder(appJobs)
	byArchiveOrder(deltaJobs)

	progress.Default.Begin("Building", progress.UnitBytes, totalSize, 0)
	defer progress.Default.End()

	var firstErr error
	var once sync.Once
	var failed atomic.Bool
	recordErr := func(err error) { once.Do(func() { firstErr = err; failed.Store(true) }) }
	staged := func(job stageJob) (bool, error) {
		valid, err := fileMatches(job.out, job.item.Size, job.item.MD5, md5.New, nil)
		if valid {
			progress.Default.Add(job.item.Size)
		}
		return valid, err
	}

	// The full files and the deltas come from separate archives, so both are
	// built at the same time.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, job := range appJobs {
			if failed.Load() {
				return
			}
			if valid, err := staged(job); err != nil {
				recordErr(err)
				return
			} else if valid {
				continue
			}
			err := verifiedOutput(job.out, job.item, func(w io.Writer) error {
				r, err := job.member.file.Open()
				if err != nil {
					return err
				}
				defer r.Close()
				_, err = io.Copy(w, r)
				return err
			})
			if err != nil {
				recordErr(fmt.Errorf("%s: %w", job.item.Name, err))
				return
			}
		}
	}()

	jobs = max(jobs, 1)
	mem := newMemBudget(max(budget, 64<<20))
	slots := make(chan struct{}, jobs)
	for _, job := range deltaJobs {
		if failed.Load() {
			break
		}
		if valid, err := staged(job); err != nil {
			recordErr(err)
			break
		} else if valid {
			continue
		}
		stat, err := os.Stat(job.source)
		if err != nil {
			recordErr(err)
			break
		}
		weight := mem.acquire(deltaMemory(stat.Size(), int64(job.member.file.UncompressedSize), job.item.Size))
		slots <- struct{}{}
		release := func() {
			<-slots
			mem.release(weight)
		}
		if failed.Load() {
			release()
			break
		}
		// Deltas are read here, one after another in archive order, so the
		// solid block is decompressed once while workers decode earlier files.
		data, err := readMember(job.member.file)
		if err != nil {
			release()
			recordErr(fmt.Errorf("%s: %w", job.item.Name, err))
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer release()
			source, err := os.ReadFile(job.source)
			if err == nil {
				err = verifiedOutput(job.out, job.item, func(w io.Writer) error {
					_, e := vcdiff.Decode(source, bytes.NewReader(data), w)
					return e
				})
			}
			if err != nil {
				recordErr(fmt.Errorf("%s: %w", job.item.Name, err))
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return names, nil
}

func readMember(f *sevenzip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data := make([]byte, 0, f.UncompressedSize)
	buf := bytes.NewBuffer(data)
	_, err = io.Copy(buf, progress.Default.Reader(r))
	return buf.Bytes(), err
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

// installJournal records an installation in progress, so one that was cut off
// (the launcher closed, Windows shut down, a crash) is rolled back on the next
// start instead of leaving a mix of old and new files.
type installJournal struct {
	Source      int      `json:"source"`
	Destination int      `json:"destination"`
	Stage       string   `json:"stage"`
	Backup      string   `json:"backup"`
	Names       []string `json:"names"`
}

func journalPath(game string) string {
	return filepath.Join(game, "-gup-", "awlauncher", "install.json")
}

func writeJournal(game string, j installJournal) error {
	data, err := json.Marshal(j)
	if err != nil {
		return err
	}
	path := journalPath(game)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return fileutil.WriteAtomic(path, data)
}

// installMu is held while game files are being swapped.
var installMu sync.Mutex

// WaitInstall blocks while game files are being swapped, so the launcher can
// exit without leaving the game half-updated.
func WaitInstall() {
	installMu.Lock()
	installMu.Unlock()
}

// RecoverInstall rolls back an installation that did not finish.
func RecoverInstall(game string) error {
	data, err := os.ReadFile(journalPath(game))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var j installJournal
	if err := json.Unmarshal(data, &j); err != nil {
		log.Printf("Ignoring a damaged install journal: %v", err)
		return os.Remove(journalPath(game))
	}
	if build, _, err := CurrentBuild(game); err == nil && build == j.Destination {
		return os.Remove(journalPath(game))
	}
	log.Printf("Patch %d -> %d did not finish installing; restoring the previous files.\n", j.Source, j.Destination)
	if err := rollbackInstall(game, j); err != nil {
		return fmt.Errorf("restore after an unfinished update: %w", err)
	}
	_ = os.RemoveAll(j.Backup)
	return os.Remove(journalPath(game))
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func moveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// rollbackInstall puts every backed-up file back and returns new files to the
// stage folder, so a retry does not need to build them again. It only looks at
// what is on disk and can run again after being interrupted itself.
func rollbackInstall(game string, j installJournal) error {
	lastName := filepath.Join("-gup-", "last.xml")
	var firstErr error
	keep := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	for i := len(j.Names) - 1; i >= 0; i-- {
		name := j.Names[i]
		target, err := SafePath(game, name)
		if err != nil {
			keep(err)
			continue
		}
		bak, err := SafePath(j.Backup, name)
		if err != nil {
			keep(err)
			continue
		}
		_ = os.Remove(target + ".awlauncher.tmp")
		staged := ""
		if name != lastName {
			staged, _ = SafePath(j.Stage, name)
		}
		hasBackup, err := exists(bak)
		if err != nil {
			keep(err)
			continue
		}
		hasTarget, err := exists(target)
		if err != nil {
			keep(err)
			continue
		}
		hasStaged := false
		if staged != "" {
			if hasStaged, err = exists(staged); err != nil {
				keep(err)
				continue
			}
		}
		if !hasBackup && (staged == "" || hasStaged || !hasTarget) {
			continue // not reached by the installation
		}
		if hasTarget {
			if staged != "" && !hasStaged {
				keep(moveFile(target, staged))
			} else {
				keep(os.Remove(target))
			}
		}
		if hasBackup {
			if err := moveFile(bak, target); err != nil {
				log.Printf("ROLLBACK ERROR %s: %v", target, err)
				keep(err)
			}
		}
	}
	return firstErr
}

func installPatch(game, stage, backup string, names []string, last []byte, patch patchInfo, manifest Manifest) error {
	updated, err := newLastXML(last, patch, manifest)
	if err != nil {
		return err
	}
	lastPath := filepath.Join(game, "-gup-", "last.xml")
	lastName := filepath.Join("-gup-", "last.xml")
	names = append(append([]string(nil), names...), lastName)
	installMu.Lock()
	defer installMu.Unlock()
	journal := installJournal{Source: patch.Source, Destination: patch.Destination, Stage: stage, Backup: backup, Names: names}
	if err := writeJournal(game, journal); err != nil {
		return err
	}
	progress.Default.Begin("Installing", progress.UnitFiles, int64(len(names)), 0)
	progress.Default.NoPause()
	defer progress.Default.End()
	fail := func(err error) error {
		if rbErr := rollbackInstall(game, journal); rbErr != nil {
			log.Printf("ROLLBACK ERROR: %v", rbErr)
			return err
		}
		_ = os.Remove(journalPath(game))
		return err
	}
	for _, name := range names {
		target, err := SafePath(game, name)
		if err != nil {
			return fail(err)
		}
		bak, err := SafePath(backup, name)
		if err != nil {
			return fail(err)
		}
		existed, err := exists(target)
		if err != nil {
			return fail(err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fail(err)
		}
		if existed {
			if err := moveFile(target, bak); err != nil {
				return fail(err)
			}
		}
		task := progress.Default.Start(name, 0)
		if name == lastName {
			tmp := lastPath + ".awlauncher.tmp"
			if err = os.WriteFile(tmp, updated, 0644); err == nil {
				err = os.Rename(tmp, target)
			}
		} else {
			src, _ := SafePath(stage, name)
			err = moveFile(src, target)
		}
		task.Done()
		progress.Default.Add(1)
		if err != nil {
			return fail(fmt.Errorf("install %s: %w", name, err))
		}
	}
	return os.Remove(journalPath(game))
}
