package progress

import (
	"log"
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
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func captureLog(t *testing.T, f func()) string {
	t.Helper()
	var out strings.Builder
	log.SetOutput(&out)
	defer log.SetOutput(os.Stderr)
	f()
	return out.String()
}

func TestBoardLogsSummary(t *testing.T) {
	b := &Board{}
	out := captureLog(t, func() {
		b.Begin("Verifying", unitBytes, 10, 0)
		b.Add(10)
		b.mu.Lock()
		b.lastLine = time.Time{}
		b.sampleLocked()
		b.mu.Unlock()
		b.End()
	})
	if strings.Contains(out, "") || !strings.Contains(out, "Verifying") || !strings.Contains(out, "100.0%") || !strings.Contains(out, "done:") {
		t.Fatalf("log = %q", out)
	}
}

func TestSnapshotFollowsPhase(t *testing.T) {
	b := &Board{}
	if b.Snapshot().Active {
		t.Fatal("snapshot is active before Begin")
	}
	captureLog(t, func() {
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

func TestResumedBytesAreNotCountedAsTransferred(t *testing.T) {
	b := &Board{}
	b.Begin("Downloading", UnitBytes, 100, 10)
	defer b.End()
	task := b.Start("file", 90)
	task.Set(60)
	task.Write(make([]byte, 25))
	if got := b.done.Load(); got != 95 {
		t.Fatalf("done %d, want 95", got)
	}
	if got := b.moved(); got != 25 {
		t.Fatalf("moved %d, want 25", got)
	}
	task.Set(0)
	task.Write(make([]byte, 10))
	if got := b.moved(); got != 35 {
		t.Fatalf("moved after a restart %d, want 35", got)
	}
}
