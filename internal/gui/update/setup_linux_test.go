package update

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAppImageUpdate(t *testing.T) {
	payload := []byte("new AppImage")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/file" {
			w.Write(payload)
			return
		}
		w.Write([]byte(`{"tag_name":"v0.2.0","html_url":"https://example.com","assets":[{"name":"` + SetupAssetName + `","browser_download_url":"http://` + r.Host + `/file","size":12}]}`))
	}))
	defer srv.Close()
	oldURL := latestReleaseURL
	defer func() { latestReleaseURL = oldURL }()
	latestReleaseURL = srv.URL

	t.Setenv("APPIMAGE", "")
	if _, err := DownloadSetup(srv.Client(), "v0.1.0"); err == nil {
		t.Fatal("a build that is not an AppImage should not update")
	}
	target := filepath.Join(t.TempDir(), "AWLauncher.AppImage")
	if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPIMAGE", target)
	next, err := DownloadSetup(srv.Client(), "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(next) != filepath.Dir(target) {
		t.Fatalf("downloaded to %s, want next to %s", next, target)
	}
	if data, _ := os.ReadFile(target); string(data) != "old" {
		t.Fatalf("the running AppImage changed before RunSetup: %q", data)
	}
}
