package progress

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestCancelNeedsAnArmedOperation(t *testing.T) {
	b := &Board{}
	if b.Cancel() || b.Cancelled() {
		t.Fatal("cancelled without an armed operation")
	}
	b.Arm()
	if !b.Cancel() || b.Cancel() || !b.Cancelled() {
		t.Fatal("Cancel should work once")
	}
	b.Arm()
	if b.Cancelled() || b.Err() != nil {
		t.Fatal("a new operation starts cancelled")
	}
}

func TestCancelStopsReadsWritesAndAPause(t *testing.T) {
	b := &Board{}
	b.Arm()
	b.Begin("Downloading", UnitBytes, 100, 0)
	defer b.End()
	task := b.Start("file", 100)
	b.Pause()
	errs := make(chan error, 1)
	go func() {
		_, err := task.Write(make([]byte, 10))
		errs <- err
	}()
	select {
	case <-errs:
		t.Fatal("work went on while paused")
	case <-time.After(50 * time.Millisecond):
	}
	b.Cancel()
	select {
	case err := <-errs:
		if !errors.Is(err, ErrCancelled) {
			t.Fatalf("write after cancel: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("a cancel did not end the pause")
	}
	if b.Pause() || b.Paused() {
		t.Fatal("a cancelled operation paused")
	}
	if _, err := b.Quiet().Reader(bytes.NewReader([]byte("x"))).Read(make([]byte, 1)); !errors.Is(err, ErrCancelled) {
		t.Fatalf("read after cancel: %v", err)
	}
	if b.Quiet().Context().Err() == nil {
		t.Fatal("the quiet board's context is not done")
	}
}
