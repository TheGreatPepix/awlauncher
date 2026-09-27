package launcher

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

var cacheEntryPattern = regexp.MustCompile(`^(payload|stage|backup)-(\d+)-(\d+)(-.*)?$`)

func staleCacheEntries(names []string, build int) []string {
	var stale []string
	for _, name := range names {
		m := cacheEntryPattern.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		dest, err := strconv.Atoi(m[3])
		if err != nil {
			continue
		}
		if dest < build || (dest == build && m[1] == "stage") {
			stale = append(stale, name)
		}
	}
	return stale
}

func cleanupCache(cacheRoot string, build int) {
	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	var freed int64
	var removed int
	for _, name := range staleCacheEntries(names, build) {
		path := filepath.Join(cacheRoot, name)
		size := dirSize(path)
		if err := os.RemoveAll(path); err != nil {
			fmt.Printf("Could not remove old patch files %s: %v\n", path, err)
			continue
		}
		freed += size
		removed++
	}
	if removed > 0 {
		fmt.Printf("Removed old patch files: %d folders, %.1f GiB freed.\n", removed, float64(freed)/gib)
	}
}

func dirSize(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}
