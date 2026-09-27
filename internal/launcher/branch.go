package launcher

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TheGreatPepix/awlauncher/internal/fileutil"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
	"github.com/TheGreatPepix/awlauncher/internal/region"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const fxDefaultBranch = "default"

const linkMinSize = 1 << 20

type fxBranchManifest struct {
	Manifest struct {
		LauncherConfiguration struct {
			LaunchFile   string `json:"launchFile"`
			HTTPDownload struct {
				DownloadBaseURI string `json:"downloadBaseUri"`
			} `json:"httpDownload"`
			ManifestURL    string `json:"manifestUrl"`
			ManifestSHA256 string `json:"manifestSha256"`
		} `json:"launcherConfiguration"`
		Release struct {
			BuildNumber  int64     `json:"buildNumber"`
			BuildVersion string    `json:"buildVersion"`
			CreatedAt    time.Time `json:"createdAt"`
		} `json:"release"`
		Artifacts []struct {
			FullSize int64 `json:"fullSize"`
		} `json:"artifacts"`
	} `json:"Manifest"`
}

func fxGetBranchManifest(client *http.Client, accessToken, branch string) (fxBranchManifest, json.RawMessage, error) {
	var m fxBranchManifest
	var raw json.RawMessage
	if err := fxAPIGet(client, "/api/v1/ftl_integration/"+fxGame+"/launcher/manifest/"+url.PathEscape(branch), accessToken, &raw); err != nil {
		return m, nil, err
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, nil, err
	}
	return m, raw, nil
}

type fxFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type branchState struct {
	Branch         string   `json:"branch"`
	Build          int64    `json:"build"`
	Version        string   `json:"version"`
	ManifestSHA256 string   `json:"manifest_sha256"`
	LaunchFile     string   `json:"launch_file"`
	Files          []fxFile `json:"files"`
	Dirty          bool     `json:"dirty,omitempty"`
	Changed        []string `json:"-"`
}

func branchInstallDir(mainRoot, branch string) string {
	return filepath.Join(filepath.Dir(mainRoot), filepath.Base(mainRoot)+" "+branch)
}

func branchStatePath(root string) string {
	return filepath.Join(root, "-gup-", "awlauncher", "branch.json")
}

func readBranchState(root string) (branchState, bool) {
	var s branchState
	data, err := os.ReadFile(branchStatePath(root))
	if err != nil || json.Unmarshal(data, &s) != nil {
		return branchState{}, false
	}
	return s, true
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, progress.Default.Reader(f)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashTask(path, label string, size int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	task := progress.Default.Start(label, size)
	defer task.Done()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(h, task), f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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

func syncBranch(client *http.Client, branch string, m fxBranchManifest, mainRoot, root string, allowMods ...bool) (branchState, error) {
	mods := len(allowMods) > 0 && allowMods[0]
	lc := m.Manifest.LauncherConfiguration
	if lc.HTTPDownload.DownloadBaseURI == "" || lc.ManifestURL == "" {
		return branchState{}, errors.New("the branch manifest has no download address")
	}
	prev, _ := readBranchState(root)
	if !prev.Dirty && prev.ManifestSHA256 != "" && strings.EqualFold(prev.ManifestSHA256, lc.ManifestSHA256) && branchFilesPresent(root, branch, prev.Files, mods) {
		return prev, nil
	}
	fmt.Printf("Preparing %s %s (build %d) in %s...\n", branch, m.Manifest.Release.BuildVersion, m.Manifest.Release.BuildNumber, root)
	data, err := fetch(client, lc.ManifestURL, 64<<20)
	if err != nil {
		return branchState{}, fmt.Errorf("file manifest: %w", err)
	}
	if lc.ManifestSHA256 != "" {
		if err := verifyHexDigest(data, lc.ManifestSHA256, "sha256"); err != nil {
			return branchState{}, fmt.Errorf("file manifest: %w", err)
		}
	}
	var manifest struct {
		Files []fxFile `json:"files"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil || len(manifest.Files) == 0 {
		return branchState{}, errors.New("unexpected file manifest")
	}

	verified := map[string]string{}
	if !prev.Dirty {
		for _, f := range prev.Files {
			verified[strings.ToLower(f.Path)] = strings.ToLower(f.SHA256)
		}
	}
	previous := map[string]string{}
	for _, f := range prev.Files {
		previous[strings.ToLower(f.Path)] = strings.ToLower(f.SHA256)
	}
	var check, download []fxFile
	var checkSize, downloadSize, linked int64
	for _, f := range manifest.Files {
		dst, err := fxFilePath(root, branch, f.Path)
		if err != nil {
			return branchState{}, err
		}
		if mods && previous[strings.ToLower(f.Path)] == strings.ToLower(f.SHA256) {
			if st, statErr := os.Stat(dst); statErr == nil && st.Mode().IsRegular() {
				continue
			}
		}
		if st, err := os.Stat(dst); err == nil && st.Size() == f.Size {
			if verified[strings.ToLower(f.Path)] == strings.ToLower(f.SHA256) {
				continue
			}
			check = append(check, f)
			checkSize += f.Size
			continue
		}
		_ = os.Remove(dst)
		if mainRoot != "" && placeFromMain(mainRoot, root, f) {
			if f.Size >= linkMinSize {
				linked += f.Size
			}
			check = append(check, f)
			checkSize += f.Size
			continue
		}
		download = append(download, f)
		downloadSize += f.Size
	}
	if linked > 0 {
		fmt.Printf("Linked %s of files shared with the main install (no extra disk space).\n", progress.FormatBytes(linked))
	}

	var mu sync.Mutex
	if len(check) > 0 {
		progress.Default.Begin("Checking", progress.UnitBytes, checkSize, 0)
		err := parallel(check, 2, func(f fxFile) error {
			dst, _ := fxFilePath(root, branch, f.Path)
			sum, err := hashTask(dst, f.Path, f.Size)
			if err == nil && strings.EqualFold(sum, f.SHA256) {
				return nil
			}
			_ = os.Remove(dst)
			mu.Lock()
			download = append(download, f)
			downloadSize += f.Size
			mu.Unlock()
			return nil
		})
		progress.Default.End()
		if err != nil {
			return branchState{}, err
		}
	}
	if len(download) > 0 {
		if branch == fxDefaultBranch {
			var names []string
			for _, f := range download {
				names = append(names, f.Path)
			}
			if err := markVKDirtyIfSharedFiles(root, names); err != nil {
				return branchState{}, err
			}
		}
		if free, err := diskFree(root); err == nil && free < downloadSize+gib {
			return branchState{}, fmt.Errorf("not enough free space: %s needed", progress.FormatBytes(downloadSize+gib))
		}
		base := strings.TrimRight(lc.HTTPDownload.DownloadBaseURI, "/") + "/data/"
		dl := &http.Client{Timeout: 2 * time.Hour}
		progress.Default.Begin("Downloading", progress.UnitBytes, downloadSize, 0)
		err := parallel(download, 4, func(f fxFile) error {
			dst, _ := fxFilePath(root, branch, f.Path)
			parts := strings.Split(f.Path, "/")
			for i := range parts {
				parts[i] = url.PathEscape(parts[i])
			}
			for attempt := 0; attempt < 2; attempt++ {
				if err := downloadOne(dl, base+strings.Join(parts, "/"), dst, f.Path, f.Size); err != nil {
					return fmt.Errorf("%s: %w", f.Path, err)
				}
				if sum, err := fileSHA256(dst); err == nil && strings.EqualFold(sum, f.SHA256) {
					return nil
				}
				_ = os.Remove(dst)
			}
			return fmt.Errorf("%s: SHA-256 mismatch after download", f.Path)
		})
		progress.Default.End()
		if err != nil {
			return branchState{}, fmt.Errorf("%w (run the launcher again to resume)", err)
		}
	}
	var changed []string
	current := map[string]bool{}
	for _, f := range manifest.Files {
		current[strings.ToLower(f.Path)] = true
	}
	for _, f := range prev.Files {
		if !current[strings.ToLower(f.Path)] {
			if branch == fxDefaultBranch {
				if err := markVKDirtyIfSharedFiles(root, []string{f.Path}); err != nil {
					return branchState{}, err
				}
			}
			if dst, err := fxFilePath(root, branch, f.Path); err == nil {
				_ = os.Remove(dst)
			}
			changed = append(changed, f.Path)
		}
	}

	state := branchState{
		Branch: branch, Build: m.Manifest.Release.BuildNumber, Version: m.Manifest.Release.BuildVersion,
		ManifestSHA256: lc.ManifestSHA256, LaunchFile: lc.LaunchFile, Files: manifest.Files,
	}
	for _, f := range download {
		changed = append(changed, f.Path)
	}
	state.Changed = changed
	out, err := json.Marshal(state)
	if err != nil {
		return branchState{}, err
	}
	if err := os.MkdirAll(filepath.Dir(branchStatePath(root)), 0755); err != nil {
		return branchState{}, err
	}
	if err := fileutil.WriteAtomic(branchStatePath(root), out); err != nil {
		return branchState{}, err
	}
	fmt.Printf("%s %s is ready.\n", branch, state.Version)
	return state, nil
}

func placeFromMain(mainRoot, root string, f fxFile) bool {
	src, err := safeGamePath(mainRoot, f.Path)
	if err != nil {
		return false
	}
	st, err := os.Stat(src)
	if err != nil || st.Size() != f.Size {
		return false
	}
	dst, err := safeGamePath(root, f.Path)
	if err != nil || os.MkdirAll(filepath.Dir(dst), 0755) != nil {
		return false
	}
	if f.Size >= linkMinSize && os.Link(src, dst) == nil {
		return true
	}
	return copyFile(src, dst) == nil
}

func fxFilePath(root, branch, name string) (string, error) {
	if branch == fxDefaultBranch && normalizedClientName(name) == "user.cfg" {
		return region.FXConfigPath(root), nil
	}
	return safeGamePath(root, name)
}

func branchFilesPresent(root, branch string, files []fxFile, allowMods bool) bool {
	for _, f := range files {
		dst, err := fxFilePath(root, branch, f.Path)
		if err != nil {
			return false
		}
		if st, err := os.Stat(dst); err != nil || !st.Mode().IsRegular() || (!allowMods && st.Size() != f.Size) {
			return false
		}
		if !allowMods && normalizedClientName(f.Path) == "user.cfg" {
			sum, err := fileSHA256(dst)
			if err != nil || !strings.EqualFold(sum, f.SHA256) {
				return false
			}
		}
	}
	return len(files) > 0
}

func playFXBranch(client *http.Client, p *prompter, acc account, mainRoot string) error {
	if mainRoot == "" {
		return errors.New("install a main client first: the branch is placed next to it")
	}
	return playFXInstall(client, p, acc, acc.Branch, branchInstallDir(mainRoot, acc.Branch), mainRoot, false, false)
}

func playFXInstall(client *http.Client, p *prompter, acc account, branch, root, mainRoot string, ask, allowMods bool) error {
	state, err := syncFXClient(client, p, acc, branch, root, mainRoot, ask, allowMods)
	if err != nil {
		return err
	}
	if branch == fxDefaultBranch {
		if err := region.Ensure(root, region.FX); err != nil {
			return fmt.Errorf("switch to the FX ID servers: %w", err)
		}
	}
	launchToken, err := fxGameToken(client, acc)
	if err != nil {
		return err
	}
	launch := state.LaunchFile
	if launch == "" {
		launch = "bin64/ArmoredWarfare.exe"
	}
	exe, err := safeGamePath(root, launch)
	if err != nil {
		return err
	}
	pid, err := startGameProcess(exe, fxLaunchArgs(fxLaunchTemplate(client), acc, launchToken), root)
	if err != nil {
		return fmt.Errorf("start the game: %w", err)
	}
	p.sayf("Game started: %s, branch %s %s, PID %d\n", acc.label(), branch, state.Version, pid)
	return nil
}

func syncFXClient(client *http.Client, p *prompter, acc account, branch, root, mainRoot string, ask, allowMods bool) (branchState, error) {
	if branch != fxDefaultBranch && isGameDir(root) {
		return branchState{}, errors.New("FX ID client folder contains a VK Play installation")
	}
	if state, ok := readBranchState(root); ok && state.Branch != branch {
		return branchState{}, fmt.Errorf("FX ID client folder belongs to branch %s", state.Branch)
	}
	if name, err := runningProcess("ArmoredWarfare.exe"); err != nil {
		return branchState{}, err
	} else if name != "" {
		return branchState{}, errors.New("the game is already running")
	}
	tokens, err := fxSession(client, acc)
	if err != nil {
		return branchState{}, err
	}
	m, _, err := fxGetBranchManifest(client, tokens.AccessToken, branch)
	if err != nil {
		return branchState{}, fmt.Errorf("branch %s: %w", branch, err)
	}
	if ask {
		if state, ok := readBranchState(root); !ok || state.Branch != branch {
			var size int64
			for _, a := range m.Manifest.Artifacts {
				size = max(size, a.FullSize)
			}
			if !p.yes(fmt.Sprintf("Download the FX ID client (%s) into %s?", progress.FormatBytes(size), root), false) {
				return branchState{}, errQuit
			}
		}
	}
	if strings.EqualFold(filepath.Clean(mainRoot), filepath.Clean(root)) {
		mainRoot = ""
	}
	state, err := syncBranch(client, branch, m, mainRoot, root, allowMods)
	if err != nil {
		return branchState{}, err
	}
	if branch == fxDefaultBranch {
		if err := markVKDirtyIfSharedFiles(root, state.Changed); err != nil {
			return branchState{}, err
		}
	}
	return state, nil
}
