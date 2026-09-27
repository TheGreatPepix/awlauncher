package progress

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFormatting(t *testing.T) {
	for _, c := range []struct{ got, want string }{
		{formatPair(143_900_000, 205_300_000), "137.2 / 195.8 MiB"},
		{formatPair(21_800_000_000, 63_600_000_000), "20.3 / 59.2 GiB"},
		{formatBytes(44 << 20), "44.0 MiB"},
		{formatDuration(16*time.Minute + 5*time.Second), "16m05s"},
		{formatDuration(2*time.Hour + 3*time.Minute), "2h03m"},
		{drawBar(0.5, 4), "[██░░]"},
		{clipLeft("gamesdk/Levels/PROMO/GAR_HOLIDAY_05/levelxml.pak", 16), "…05/levelxml.pak"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	f()
	os.Stdout = old
	w.Close()
	data, _ := io.ReadAll(r)
	return string(data)
}

func TestBoardRedrawsInPlace(t *testing.T) {
	b := &progressBoard{live: true}
	out := captureStdout(t, func() {
		b.Begin("Downloading", unitBytes, 300<<20, 100<<20)
		task := b.Start("gamesdk/textures_hi-0134.pak", 200<<20)
		task.Add(50 << 20)
		b.mu.Lock()
		b.renderLocked()
		b.renderLocked()
		b.mu.Unlock()
		task.Add(150 << 20)
		task.Done()
		b.End()
	})
	for _, want := range []string{
		"Downloading ", "[██████████", "50.0%", "150.0 / 300.0 MiB",
		"textures_hi-0134.pak", "25.0%", "50.0 / 200.0 MiB",
		"\x1b[2A",
		"done: 300.0 MiB",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in output %q", want, out)
		}
	}
	tail := out[strings.LastIndex(out, "\r"):]
	if strings.Contains(tail, "textures_hi") || !strings.Contains(tail, "done:") {
		t.Errorf("final redraw = %q", tail)
	}
}

func TestBoardPlainWhenRedirected(t *testing.T) {
	b := &progressBoard{live: false}
	out := captureStdout(t, func() {
		b.Begin("Verifying", unitBytes, 10, 0)
		b.Add(10)
		b.mu.Lock()
		b.renderLocked()
		b.mu.Unlock()
		b.End()
	})
	if strings.Contains(out, "\x1b") || !strings.Contains(out, "Verifying") || !strings.Contains(out, "done:") {
		t.Fatalf("plain output = %q", out)
	}
}

func TestSnapshotFollowsPhase(t *testing.T) {
	b := &progressBoard{live: false}
	if b.Snapshot().Active {
		t.Fatal("snapshot is active before Begin")
	}
	captureStdout(t, func() {
		b.Begin("Checking", unitFiles, 40, 10)
		task := b.Start("bin64/ArmoredWarfare.exe", 5)
		task.Add(3)
		s := b.Snapshot()
		if !s.Active || s.Title != "Checking" || !s.Files || s.Done != 13 || s.Total != 40 {
			t.Errorf("snapshot = %+v", s)
		}
		if len(s.Tasks) != 1 || s.Tasks[0].Label != "bin64/ArmoredWarfare.exe" || s.Tasks[0].Done != 3 || s.Tasks[0].Total != 5 {
			t.Errorf("tasks = %+v", s.Tasks)
		}
		task.Done()
		b.End()
	})
	if b.Snapshot().Active {
		t.Fatal("snapshot is active after End")
	}
}
