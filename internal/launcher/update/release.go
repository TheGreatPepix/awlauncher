package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

const releasesPage = "https://github.com/TheGreatPepix/awlauncher/releases"

var latestReleaseURL = "https://api.github.com/repos/TheGreatPepix/awlauncher/releases/latest"

type releaseAsset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}

type release struct {
	Tag       string         `json:"tag_name"`
	Page      string         `json:"html_url"`
	Notes     string         `json:"body"`
	Published time.Time      `json:"published_at"`
	Draft     bool           `json:"draft"`
	Assets    []releaseAsset `json:"assets"`
}

func (r release) asset(name string) (releaseAsset, bool) {
	for _, a := range r.Assets {
		if strings.EqualFold(a.Name, name) {
			return a, true
		}
	}
	return releaseAsset{}, false
}

func latestRelease(client *http.Client, current string) (release, error) {
	req, err := http.NewRequest(http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "AWLauncher/"+current)
	resp, err := client.Do(req)
	if err != nil {
		return release{}, fmt.Errorf("cannot reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return release{}, errors.New("no AWLauncher release is published yet")
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return release{}, errors.New("GitHub limits requests from this address; try again in an hour")
	case resp.StatusCode != http.StatusOK:
		return release{}, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var r release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&r); err != nil {
		return release{}, fmt.Errorf("cannot read the release from GitHub: %w", err)
	}
	if r.Tag == "" || r.Draft {
		return release{}, errors.New("GitHub returned no release")
	}
	if r.Page == "" {
		r.Page = releasesPage + "/tag/" + r.Tag
	}
	return r, nil
}

func parseVersion(v string) (core [3]int, suffix string, ok bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	main, suffix, _ := strings.Cut(v, "-")
	main, _, _ = strings.Cut(main, "+")
	parts := strings.Split(main, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return core, "", false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return core, "", false
		}
		core[i] = n
	}
	return core, suffix, true
}

func newerRelease(tag, current string) bool {
	t, tagSuffix, ok := parseVersion(tag)
	if !ok || tagSuffix != "" {
		return false
	}
	c, _, ok := parseVersion(current)
	if !ok {
		return false
	}
	for i := range t {
		if t[i] != c[i] {
			return t[i] > c[i]
		}
	}
	return false
}

func ReleaseBuild(v string) bool {
	_, _, ok := parseVersion(v)
	return ok
}

func downloadAsset(client *http.Client, a releaseAsset, path string) error {
	resp, err := client.Get(a.URL)
	if err != nil {
		return fmt.Errorf("download %s: %w", a.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", a.Name, resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	task := progress.Default.Start(a.Name, a.Size)
	defer task.Done()
	sum := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, sum, task), io.LimitReader(resp.Body, a.Size+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n != a.Size {
		err = fmt.Errorf("download %s: got %d bytes instead of %d", a.Name, n, a.Size)
	}
	if want, ok := strings.CutPrefix(a.Digest, "sha256:"); err == nil && ok && !strings.EqualFold(want, hex.EncodeToString(sum.Sum(nil))) {
		err = fmt.Errorf("download %s: the checksum does not match", a.Name)
	}
	if err != nil {
		os.Remove(path)
		return err
	}
	return nil
}

type Info struct {
	Type      string `json:"type"`
	Current   string `json:"current"`
	Latest    string `json:"latest,omitempty"`
	Page      string `json:"page"`
	Notes     string `json:"notes,omitempty"`
	Published string `json:"published,omitempty"`
	Available bool   `json:"available"`
	CanApply  bool   `json:"canApply"`
	Quiet     bool   `json:"quiet,omitempty"`
	Error     string `json:"error,omitempty"`
}

func Check(client *http.Client, current string) Info {
	info := Info{Type: "update", Current: current, Page: releasesPage + "/latest"}
	r, err := latestRelease(client, current)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	info.Latest, info.Page, info.Notes = r.Tag, r.Page, strings.TrimSpace(r.Notes)
	if !r.Published.IsZero() {
		info.Published = r.Published.Local().Format("02.01.2006")
	}
	info.Available = newerRelease(r.Tag, current)
	_, hasExe := r.asset(SelfAssetName)
	info.CanApply = info.Available && hasExe
	return info
}
