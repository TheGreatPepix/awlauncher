package gui

import (
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

type endless struct{}

func (endless) Read(p []byte) (int, error) { return len(p), nil }

func TestCancelStopsADownloadAndRemovesTheClientWhenAsked(t *testing.T) {
	for _, tc := range []struct {
		kind, value, message string
		removed              bool
	}{
		{launcher.CancelUpdate, "keep", "Download stopped", false},
		{launcher.CancelUpdate, "remove", "Download cancelled, the client is removed", true},
		{launcher.CancelInstall, "keep", "Download cancelled, the client is removed", true},
	} {
		g, h := testApp(t)
		g.pageReady.Store(true)
		var removed, writing atomic.Bool
		g.run(launcher.Operation{Title: "Updating VK Play", Game: true, Cancel: tc.kind}, "", func(*launcher.Session) (bool, error) {
			writing.Store(true)
			_, err := io.Copy(io.Discard, progress.Default.Reader(endless{}))
			if removed.Load() {
				t.Error("the client was removed while the download ran")
			}
			return false, g.afterCancel(err, func() error { removed.Store(true); return nil })
		})
		for !writing.Load() {
			time.Sleep(time.Millisecond)
		}
		g.onMessage(`{"cmd":"cancel","value":"` + tc.value + `"}`)
		done := waitDone(t, h)
		if done["status"] != "cancelled" || done["message"] != tc.message || removed.Load() != tc.removed {
			t.Errorf("%s/%s: done = %v, removed = %v", tc.kind, tc.value, done, removed.Load())
		}
	}
}

func TestCancelIgnoresOperationsThatCannotBeCancelled(t *testing.T) {
	g, h := testApp(t)
	g.pageReady.Store(true)
	release := make(chan struct{})
	g.run(launcher.Operation{Title: "Checking VK Play", Game: true}, "", func(*launcher.Session) (bool, error) {
		<-release
		return false, g.afterCancel(nil, func() error { t.Error("removed"); return nil })
	})
	g.onMessage(`{"cmd":"cancel","value":"remove"}`)
	close(release)
	if done := waitDone(t, h); done["status"] != "ok" {
		t.Fatalf("done = %v", done)
	}
}

func waitDone(t *testing.T, h *fakeHost) map[string]any {
	t.Helper()
	for range 400 {
		h.mu.Lock()
		for _, e := range h.events {
			if e["type"] == "done" {
				h.events = nil
				h.mu.Unlock()
				return e
			}
		}
		h.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the operation did not end")
	return nil
}
