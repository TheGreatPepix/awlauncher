package update

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewerRelease(t *testing.T) {
	cases := []struct {
		tag, current string
		want         bool
	}{
		{"v0.1.2", "v0.1.1", true},
		{"v0.2.0", "v0.1.9", true},
		{"v1.0.0", "v0.99.99", true},
		{"v0.1.10", "v0.1.9", true},
		{"v0.1.1", "v0.1.1", false},
		{"v0.1.0", "v0.1.1", false},
		{"v0.1.2", "v0.1.1-3-g56a3122", true},
		{"v0.1.1", "v0.1.1-3-g56a3122", false},
		{"v0.1.1", "v0.1.1-3-g56a3122-dirty", false},
		{"v0.2", "v0.1.5", true},
		{"0.2.0", "v0.1.5", true},
		{"v0.2.0-rc1", "v0.1.5", false},
		{"v0.2.0", "dev", false},
		{"nightly", "v0.1.5", false},
	}
	for _, c := range cases {
		if got := newerRelease(c.tag, c.current); got != c.want {
			t.Errorf("newerRelease(%q, %q) = %v, want %v", c.tag, c.current, got, c.want)
		}
	}
	if ReleaseBuild("dev") || !ReleaseBuild("v0.1.1") || !ReleaseBuild("v0.1.1-3-g56a3122") {
		t.Error("releaseBuild misreads the build version")
	}
}

func TestCheckForUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("the request has no User-Agent, which GitHub requires")
		}
		w.Write([]byte(`{"tag_name":"v0.2.0","html_url":"https://github.com/TheGreatPepix/awlauncher/releases/tag/v0.2.0",
			"body":"Notes\r\n","published_at":"2026-09-27T10:00:00Z",
			"assets":[{"name":"` + SelfAssetName + `","browser_download_url":"https://example.com/` + SelfAssetName + `","size":10}]}`))
	}))
	defer srv.Close()
	oldURL := latestReleaseURL
	defer func() { latestReleaseURL = oldURL }()
	latestReleaseURL = srv.URL

	info := Check(srv.Client(), "v0.1.1")
	if info.Error != "" || !info.Available || !info.CanApply || info.Latest != "v0.2.0" || info.Notes != "Notes" || info.Published == "" {
		t.Fatalf("v0.1.1: %+v", info)
	}
	if info := Check(srv.Client(), "v0.2.0"); info.Available || info.CanApply || info.Error != "" {
		t.Fatalf("v0.2.0: %+v", info)
	}
	if info := Check(srv.Client(), "dev"); info.Available || info.Latest != "v0.2.0" {
		t.Fatalf("dev: %+v", info)
	}
}

func TestLatestReleaseErrors(t *testing.T) {
	status := http.StatusNotFound
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
	defer srv.Close()
	oldURL := latestReleaseURL
	defer func() { latestReleaseURL = oldURL }()
	latestReleaseURL = srv.URL
	for _, status = range []int{http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError} {
		if _, err := latestRelease(srv.Client(), "dev"); err == nil {
			t.Errorf("status %d: no error", status)
		}
	}
}

func TestDownloadAsset(t *testing.T) {
	payload := []byte("MZ new launcher")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(payload) }))
	defer srv.Close()
	sum := sha256.Sum256(payload)
	dir := t.TempDir()
	path := filepath.Join(dir, "AWLauncher-next.exe")

	good := releaseAsset{Name: "AWLauncher.exe", URL: srv.URL, Size: int64(len(payload)), Digest: "sha256:" + hex.EncodeToString(sum[:])}
	if err := downloadAsset(srv.Client(), good, path); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != string(payload) {
		t.Fatalf("saved %q", data)
	}

	for _, bad := range []releaseAsset{
		{Name: "AWLauncher.exe", URL: srv.URL, Size: int64(len(payload)), Digest: "sha256:" + hex.EncodeToString(make([]byte, 32))},
		{Name: "AWLauncher.exe", URL: srv.URL, Size: int64(len(payload)) + 1},
		{Name: "AWLauncher.exe", URL: srv.URL, Size: int64(len(payload)) - 1},
	} {
		if err := downloadAsset(srv.Client(), bad, path); err == nil {
			t.Errorf("%+v: no error", bad)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%+v: the broken download is left", bad)
		}
	}
}
