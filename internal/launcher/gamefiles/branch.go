package gamefiles

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/fileutil"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/catalog"
)

const DefaultBranch = "default"
const linkMinSize = 1 << 20

type BranchFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type BranchState struct {
	Branch         string       `json:"branch"`
	Build          int64        `json:"build"`
	Version        string       `json:"version"`
	ManifestSHA256 string       `json:"manifest_sha256"`
	LaunchFile     string       `json:"launch_file"`
	Files          []BranchFile `json:"files"`
	Dirty          bool         `json:"dirty,omitempty"`
}

func BranchDir(mainRoot, branch string) string {
	return filepath.Join(filepath.Dir(mainRoot), filepath.Base(mainRoot)+" "+branch)
}
func branchStatePath(root string) string {
	return filepath.Join(root, "-gup-", "awlauncher", "branch.json")
}
func ReadBranchState(root string) (BranchState, bool) {
	var s BranchState
	data, err := os.ReadFile(branchStatePath(root))
	if err != nil || json.Unmarshal(data, &s) != nil {
		return BranchState{}, false
	}
	return s, true
}

type BranchRelease struct {
	Name            string
	Version         string
	Build           int64
	ManifestURL     string
	ManifestSHA256  string
	DownloadBaseURI string
	LaunchFile      string
}

func SyncBranch(client *http.Client, rel BranchRelease, mainRoot, root string, allowMods bool) (BranchState, error) {
	branch := rel.Name
	if rel.DownloadBaseURI == "" || rel.ManifestURL == "" {
		return BranchState{}, errors.New("the branch manifest has no download address")
	}
	prev, _ := ReadBranchState(root)
	if !prev.Dirty && prev.ManifestSHA256 != "" && strings.EqualFold(prev.ManifestSHA256, rel.ManifestSHA256) && branchFilesPresent(root, prev.Files, allowMods) {
		return prev, nil
	}
	fmt.Printf("Preparing %s %s (build %d) in %s...\n", branch, rel.Version, rel.Build, root)
	data, err := catalog.Fetch(client, rel.ManifestURL, 64<<20)
	if err != nil {
		return BranchState{}, fmt.Errorf("file manifest: %w", err)
	}
	if rel.ManifestSHA256 != "" {
		if err := catalog.VerifyHexDigest(data, rel.ManifestSHA256, "sha256"); err != nil {
			return BranchState{}, fmt.Errorf("file manifest: %w", err)
		}
	}
	var manifest struct {
		Files []BranchFile `json:"files"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil || len(manifest.Files) == 0 {
		return BranchState{}, errors.New("unexpected file manifest")
	}

	verified := map[string]string{}
	if !prev.Dirty {
		for _, f := range prev.Files {
			if normalizedClientName(f.Path) != "user.cfg" {
				verified[strings.ToLower(f.Path)] = strings.ToLower(f.SHA256)
			}
		}
	}
	previous := map[string]string{}
	for _, f := range prev.Files {
		previous[strings.ToLower(f.Path)] = strings.ToLower(f.SHA256)
	}
	base := strings.TrimRight(rel.DownloadBaseURI, "/") + "/data/"
	set := fileSet{NewHash: sha256.New, Algo: "SHA-256"}
	for _, f := range manifest.Files {
		parts := strings.Split(f.Path, "/")
		for i := range parts {
			parts[i] = url.PathEscape(parts[i])
		}
		set.Files = append(set.Files, remoteFile{Path: f.Path, Size: f.Size, Hash: f.SHA256, URL: base + strings.Join(parts, "/")})
	}
	opt := syncOptions{
		Jobs:    4,
		Reserve: gib,
		Trusted: func(f remoteFile) bool { return verified[strings.ToLower(f.Path)] == strings.ToLower(f.Hash) },
		Keep: func(f remoteFile) bool {
			return allowMods && previous[strings.ToLower(f.Path)] == strings.ToLower(f.Hash)
		},
	}
	if mainRoot != "" {
		opt.Reuse = func(f remoteFile, dst string) bool { return linkFromMain(mainRoot, f, dst) }
	}
	if _, err := syncFiles(root, set, opt); err != nil {
		return BranchState{}, err
	}
	current := map[string]bool{}
	for _, f := range manifest.Files {
		current[strings.ToLower(f.Path)] = true
	}
	for _, f := range prev.Files {
		if !current[strings.ToLower(f.Path)] {
			if dst, err := SafePath(root, f.Path); err == nil {
				_ = os.Remove(dst)
			}
		}
	}

	state := BranchState{
		Branch: branch, Build: rel.Build, Version: rel.Version,
		ManifestSHA256: rel.ManifestSHA256, LaunchFile: rel.LaunchFile, Files: manifest.Files,
	}
	out, err := json.Marshal(state)
	if err != nil {
		return BranchState{}, err
	}
	if err := os.MkdirAll(filepath.Dir(branchStatePath(root)), 0755); err != nil {
		return BranchState{}, err
	}
	if err := fileutil.WriteAtomic(branchStatePath(root), out); err != nil {
		return BranchState{}, err
	}
	fmt.Printf("%s %s is ready.\n", branch, state.Version)
	return state, nil
}
func linkFromMain(mainRoot string, f remoteFile, dst string) bool {
	src, err := SafePath(mainRoot, f.Path)
	if err != nil {
		return false
	}
	st, err := os.Stat(src)
	if err != nil || st.Size() != f.Size || os.MkdirAll(filepath.Dir(dst), 0755) != nil {
		return false
	}
	if f.Size >= linkMinSize && os.Link(src, dst) == nil {
		return true
	}
	return copyFile(src, dst) == nil
}

func branchFilesPresent(root string, files []BranchFile, allowMods bool) bool {
	for _, f := range files {
		dst, err := SafePath(root, f.Path)
		if err != nil {
			return false
		}
		if st, err := os.Stat(dst); err != nil || !st.Mode().IsRegular() || (!allowMods && st.Size() != f.Size) {
			return false
		}
		if !allowMods && normalizedClientName(f.Path) == "user.cfg" {
			if ok, err := fileMatches(dst, f.Size, f.SHA256, sha256.New, nil); err != nil || !ok {
				return false
			}
		}
	}
	return len(files) > 0
}

func MarkBranchDirty(root string) error {
	state, ok := ReadBranchState(root)
	if !ok {
		return nil
	}
	state.Dirty = true
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return fileutil.WriteAtomic(branchStatePath(root), data)
}
