package workers

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestEachRunsEveryItem(t *testing.T) {
	var sum atomic.Int64
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if err := Each(items, 3, func(n int) error { sum.Add(int64(n)); return nil }); err != nil {
		t.Fatal(err)
	}
	if sum.Load() != 55 {
		t.Fatalf("sum %d", sum.Load())
	}
}

func TestEachKeepsToTheJobLimit(t *testing.T) {
	var running, peak atomic.Int32
	items := make([]int, 20)
	Each(items, 4, func(int) error {
		n := running.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		running.Add(-1)
		return nil
	})
	if peak.Load() > 4 {
		t.Fatalf("%d jobs ran at once", peak.Load())
	}
}

func TestEachStopsStartingItemsAfterAnError(t *testing.T) {
	boom := errors.New("boom")
	var started atomic.Int32
	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}
	err := Each(items, 2, func(n int) error {
		started.Add(1)
		if n == 3 {
			return boom
		}
		time.Sleep(2 * time.Millisecond)
		return nil
	})
	if !errors.Is(err, boom) {
		t.Fatalf("error %v", err)
	}
	if started.Load() > 10 {
		t.Fatalf("%d items started after the failure", started.Load())
	}
}
