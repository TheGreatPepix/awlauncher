package progress

import (
	"bytes"
	"io"
	"testing"
	"time"
)

func TestPauseHoldsWorkUntilResume(t *testing.T) {
	b := &Board{}
	b.Begin("Downloading", UnitBytes, 100, 0)
	defer b.End()
	task := b.Start("file", 100)
	if !b.Pause() || b.Pause() {
		t.Fatal("Pause should work once")
	}
	if b.Pauses() != 1 || !b.Snapshot().Paused || !b.Snapshot().Pausable {
		t.Fatalf("snapshot: %+v", b.Snapshot())
	}
	wrote := make(chan struct{})
	go func() {
		task.Write(make([]byte, 10))
		b.Add(5)
		close(wrote)
	}()
	select {
	case <-wrote:
		t.Fatal("work went on while paused")
	case <-time.After(100 * time.Millisecond):
	}
	if !b.Resume() || b.Resume() {
		t.Fatal("Resume should work once")
	}
	select {
	case <-wrote:
	case <-time.After(time.Second):
		t.Fatal("work did not go on after Resume")
	}
	if got := b.done.Load(); got != 15 {
		t.Fatalf("done = %d, want 15", got)
	}
}

func TestNoPauseAndReader(t *testing.T) {
	b := &Board{}
	b.Begin("Installing", UnitFiles, 3, 0)
	b.NoPause()
	b.Pause()
	done := make(chan struct{})
	go func() {
		b.Add(1)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("work that must not pause was held")
	}
	if b.Snapshot().Pausable {
		t.Fatal("the snapshot offers a pause")
	}
	b.End()

	b.Begin("Verifying", UnitBytes, 4, 0)
	defer b.End()
	read := make(chan []byte)
	go func() {
		data, _ := io.ReadAll(b.Reader(bytes.NewReader([]byte("data"))))
		read <- data
	}()
	select {
	case <-read:
		t.Fatal("a read went on while paused")
	case <-time.After(100 * time.Millisecond):
	}
	b.Resume()
	if data := <-read; string(data) != "data" {
		t.Fatalf("read %q", data)
	}
}

func TestQuietBoardFollowsThePauseAndStaysOffScreen(t *testing.T) {
	b := &Board{}
	b.Begin("Building", UnitBytes, 100, 0)
	defer b.End()
	quiet := b.Quiet()
	task := quiet.Start("next patch", 50)
	b.Pause()
	if quiet.Pauses() != b.Pauses() {
		t.Fatal("the quiet board does not see the pause")
	}
	wrote := make(chan struct{})
	go func() {
		task.Write(make([]byte, 20))
		close(wrote)
	}()
	select {
	case <-wrote:
		t.Fatal("quiet work went on while paused")
	case <-time.After(100 * time.Millisecond):
	}
	b.Resume()
	select {
	case <-wrote:
	case <-time.After(time.Second):
		t.Fatal("quiet work did not go on after Resume")
	}
	task.Done()
	if s := b.Snapshot(); s.Done != 0 || len(s.Tasks) != 0 {
		t.Fatalf("quiet work shows up on the board: %+v", s)
	}
}
