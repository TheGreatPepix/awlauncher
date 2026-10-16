package gamefiles

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/fileutil"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
)

func pendingPath(root string) string {
	return filepath.Join(root, "-gup-", "awlauncher", "pending.json")
}

// markPending records the files an install is about to write, so an install
// that is cancelled before it records the client can still be removed.
func markPending(root string, files []remoteFile) error {
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Path)
	}
	data, err := json.Marshal(names)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(pendingPath(root)), 0755); err != nil {
		return err
	}
	return fileutil.WriteAtomic(pendingPath(root), data)
}

func clearPending(root string) { _ = os.Remove(pendingPath(root)) }

func readPending(root string) []string {
	var names []string
	if data, err := os.ReadFile(pendingPath(root)); err == nil {
		_ = json.Unmarshal(data, &names)
	}
	return names
}

func removeEmptyDirs(root string) {
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		_ = os.Remove(dir)
	}
}

func CountFiles(root string) (int, int64) {
	var n int
	var size int64
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
			if info, err := d.Info(); err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return n, size
}

func RemovableFolder(dir string) bool {
	dir = filepath.Clean(dir)
	if filepath.Dir(dir) == dir || !filepath.IsAbs(dir) {
		return false
	}
	for _, protected := range platform.ProtectedDirs() {
		if protected == "" {
			continue
		}
		protected = filepath.Clean(protected)
		if strings.EqualFold(dir, protected) || strings.HasPrefix(strings.ToLower(protected), strings.ToLower(dir)+string(filepath.Separator)) {
			return false
		}
	}
	if home := platform.HomeDir(); home != "" && strings.EqualFold(filepath.Dir(dir), filepath.Clean(home)) {
		return false
	}
	return true
}

func RemoveClientFiles(dir string) error {
	names := append([]string{"user.cfg"}, readPending(dir)...)
	if IsVKInstall(dir) {
		if build, _, err := CurrentBuild(dir); err == nil {
			if inv, err := loadClientInventory(dir, build); err == nil {
				for _, f := range inv.Files {
					names = append(names, f.Name)
				}
			}
		}
	}
	if st, ok := ReadBranchState(dir); ok {
		for _, f := range st.Files {
			names = append(names, f.Path)
		}
	}
	for _, name := range names {
		path, err := SafePath(dir, name)
		if err != nil {
			continue
		}
		for _, file := range []string{path, path + ".part"} {
			if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	if err := os.RemoveAll(filepath.Join(dir, "-gup-")); err != nil {
		return err
	}
	removeEmptyDirs(dir)
	return nil
}
