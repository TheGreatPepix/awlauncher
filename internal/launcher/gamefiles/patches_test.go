package gamefiles

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/download"
)

func testTorrent(name string, files map[string]string, order []string) []byte {
	var stream string
	var list strings.Builder
	for _, f := range order {
		stream += files[f]
		fmt.Fprintf(&list, "d6:lengthi%de4:pathl%d:%see", len(files[f]), len(f), f)
	}
	const piece = 16
	var pieces []byte
	for i := 0; i < len(stream); i += piece {
		sum := sha1.Sum([]byte(stream[i:min(i+piece, len(stream))]))
		pieces = append(pieces, sum[:]...)
	}
	seed := "http://pkg.dl.mail.ru/seed/"
	info := fmt.Sprintf("d5:filesl%se4:name%d:%s12:piece lengthi%de6:pieces%d:%se", list.String(), len(name), name, piece, len(pieces), pieces)
	return []byte(fmt.Sprintf("d4:info%s8:url-list%d:%se", info, len(seed), seed))
}

func TestPrefetchLoadsTheNextPatchAndStopsOnRequest(t *testing.T) {
	files := map[string]string{"a.bin": "the next patch, first part", "b.bin": "and its second part"}
	quick := testTorrent("quick", files, []string{"a.bin", "b.bin"})
	slow := testTorrent("slow", map[string]string{"s.bin": "never finishes"}, []string{"s.bin"})
	release := make(chan struct{})
	defer close(release)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/quick.torrent":
			w.Write(quick)
		case r.URL.Path == "/slow.torrent":
			w.Write(slow)
		case strings.HasPrefix(r.URL.Path, "/seed/quick/"):
			http.ServeContent(w, r, "", time.Time{}, strings.NewReader(files[strings.TrimPrefix(r.URL.Path, "/seed/quick/")]))
		case r.URL.Path == "/seed/slow/s.bin":
			w.Header().Set("Content-Length", "14")
			w.Write([]byte("nev"))
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-release:
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	old := download.Transport
	t.Cleanup(func() { download.Transport = old })
	download.Transport = old.Clone()
	download.Transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}
	sign := func(data []byte) string { sum := sha1.Sum(data); return hex.EncodeToString(sum[:]) }
	cache := t.TempDir()

	next := patchInfo{Source: 1, Destination: 2, TorrentURL: srv.URL + "/quick.torrent", TorrentSHA1: sign(quick)}
	pre := startPrefetch(srv.Client(), cache, next)
	if pre == nil || pre.done == nil {
		t.Fatalf("no background download started: %+v", pre)
	}
	<-pre.done
	meta, ok := pre.stop()
	if !ok || meta.Name != "quick" {
		t.Fatalf("stop returned %v, %+v", ok, meta)
	}
	for name, want := range files {
		if data, _ := os.ReadFile(filepath.Join(patchDir(cache, next), name)); string(data) != want {
			t.Fatalf("%s holds %q", name, data)
		}
	}
	if again := startPrefetch(srv.Client(), cache, next); again == nil || again.done != nil {
		t.Fatalf("a complete payload started another download: %+v", again)
	}

	stuck := patchInfo{Source: 2, Destination: 3, TorrentURL: srv.URL + "/slow.torrent", TorrentSHA1: sign(slow)}
	pre = startPrefetch(srv.Client(), cache, stuck)
	if pre == nil || pre.done == nil {
		t.Fatal("no background download started for the slow patch")
	}
	time.Sleep(100 * time.Millisecond)
	start := time.Now()
	if _, ok := pre.stop(); !ok {
		t.Fatal("stop lost the torrent")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("stopping took %s", elapsed)
	}

	var none *prefetch
	if _, ok := none.stop(); ok {
		t.Fatal("stopping nothing returned a torrent")
	}
}
