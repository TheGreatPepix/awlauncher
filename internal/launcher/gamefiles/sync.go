package gamefiles

import (
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/download"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

type remoteFile struct {
	Path     string
	Size     int64
	Hash     string
	URL      string
	Modified int64
}

type fileSet struct {
	Files   []remoteFile
	NewHash func() hash.Hash
	Algo    string
}

type syncOptions struct {
	Trusted func(f remoteFile) bool
	Keep    func(f remoteFile) bool
	Reuse   func(f remoteFile, dst string) bool
	Confirm func(bad []remoteFile, size int64) error
	Jobs    int
	Reserve int64
}

func syncFiles(root string, set fileSet, opt syncOptions) ([]remoteFile, error) {
	var check, fetch []remoteFile
	var checkSize, reused int64
	for _, f := range set.Files {
		dst, err := SafePath(root, f.Path)
		if err != nil {
			return nil, err
		}
		st, statErr := os.Stat(dst)
		if statErr == nil && st.Mode().IsRegular() && opt.Keep != nil && opt.Keep(f) {
			continue
		}
		if statErr == nil && st.Mode().IsRegular() && st.Size() == f.Size {
			if opt.Trusted != nil && opt.Trusted(f) {
				continue
			}
			check = append(check, f)
			checkSize += f.Size
			continue
		}
		if opt.Reuse != nil {
			_ = os.Remove(dst)
			if opt.Reuse(f, dst) {
				if f.Size >= linkMinSize {
					reused += f.Size
				}
				check = append(check, f)
				checkSize += f.Size
				continue
			}
		}
		fetch = append(fetch, f)
	}
	if reused > 0 {
		log.Printf("Linked %s of files shared with the main install (no extra disk space).\n", progress.FormatBytes(reused))
	}

	if len(check) > 0 {
		var mu sync.Mutex
		progress.Default.Begin("Checking", progress.UnitBytes, checkSize, 0)
		err := parallel(check, 2, func(f remoteFile) error {
			dst, _ := SafePath(root, f.Path)
			task := progress.Default.Start(f.Path, f.Size)
			ok, err := fileMatches(dst, f.Size, f.Hash, set.NewHash, task)
			task.Done()
			if err != nil {
				return fmt.Errorf("check %s: %w", f.Path, err)
			}
			if !ok {
				mu.Lock()
				fetch = append(fetch, f)
				mu.Unlock()
			}
			return nil
		})
		progress.Default.End()
		if err != nil {
			return nil, err
		}
	}
	if len(fetch) == 0 {
		return nil, nil
	}

	var total int64
	for _, f := range fetch {
		total += f.Size
	}
	if opt.Confirm != nil {
		if err := opt.Confirm(fetch, total); err != nil {
			return nil, err
		}
	}
	if free, err := platform.DiskFree(root); err == nil && free < total+opt.Reserve {
		return nil, fmt.Errorf("not enough free space: %s needed", progress.FormatBytes(total+opt.Reserve))
	}
	jobs := opt.Jobs
	if jobs < 1 {
		jobs = 1
	}
	log.Printf("Downloading %d files (%s)...\n", len(fetch), progress.FormatBytes(total))
	client := &http.Client{Timeout: 2 * time.Hour}
	progress.Default.Begin("Downloading", progress.UnitBytes, total, 0)
	err := parallel(fetch, jobs, func(f remoteFile) error {
		dst, _ := SafePath(root, f.Path)
		for attempt := 0; attempt < 2; attempt++ {
			if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if err := download.File(client, f.URL, dst, f.Path, f.Size); err != nil {
				return fmt.Errorf("%s: %w", f.Path, err)
			}
			if ok, err := fileMatches(dst, f.Size, f.Hash, set.NewHash, nil); err == nil && ok {
				if f.Modified > 0 {
					m := time.Unix(f.Modified, 0)
					_ = os.Chtimes(dst, m, m)
				}
				return nil
			}
		}
		_ = os.Remove(dst)
		return fmt.Errorf("%s: %s mismatch after download", f.Path, set.Algo)
	})
	progress.Default.End()
	if err != nil {
		return nil, fmt.Errorf("%w (run the launcher again to resume)", err)
	}
	return fetch, nil
}

func fileMatches(path string, size int64, want string, newHash func() hash.Hash, track io.Writer) (bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return false, err
	}
	if !st.Mode().IsRegular() || st.Size() != size {
		return false, nil
	}
	h := newHash()
	w := io.Writer(h)
	if track != nil {
		w = io.MultiWriter(h, track)
	}
	if _, err := io.Copy(w, progress.Default.Reader(f)); err != nil {
		return false, err
	}
	return strings.EqualFold(hex.EncodeToString(h.Sum(nil)), want), nil
}

func parallel[T any](items []T, jobs int, fn func(T) error) error {
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	var once sync.Once
	var first error
	for _, it := range items {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if err := fn(it); err != nil {
				once.Do(func() { first = err })
			}
		}()
	}
	wg.Wait()
	return first
}
