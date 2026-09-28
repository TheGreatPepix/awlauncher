package progress

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const gib = 1 << 30

type progressUnit int

const (
	unitBytes progressUnit = iota
	unitFiles
)

const (
	UnitBytes = unitBytes
	UnitFiles = unitFiles
)

type progressTask struct {
	label string
	total int64
	done  atomic.Int64
	board *progressBoard
}

func (t *progressTask) Write(p []byte) (int, error) {
	t.board.Wait()
	t.Add(int64(len(p)))
	return len(p), nil
}

func (t *progressTask) Add(n int64) {
	t.board.Wait()
	t.done.Add(n)
	t.board.done.Add(n)
}

func (t *progressTask) Set(v int64) {
	old := t.done.Swap(v)
	t.board.done.Add(v - old)
}

func (t *progressTask) Done() { t.board.finish(t) }

type progressSample struct {
	at   time.Time
	done int64
}

type progressBoard struct {
	mu        sync.Mutex
	active    bool
	pausable  atomic.Bool
	gate      pauseGate
	title     string
	unit      progressUnit
	total     int64
	done      atomic.Int64
	startDone int64
	started   time.Time
	lastLine  time.Time
	tasks     []*progressTask
	samples   []progressSample
	stop      chan struct{}
	stopped   chan struct{}
}

var Default = &progressBoard{}

type Snapshot struct {
	Active   bool           `json:"active"`
	Title    string         `json:"title,omitempty"`
	Files    bool           `json:"files,omitempty"`
	Done     int64          `json:"done"`
	Total    int64          `json:"total"`
	Speed    float64        `json:"speed,omitempty"`
	Tasks    []TaskSnapshot `json:"tasks,omitempty"`
	Paused   bool           `json:"paused,omitempty"`
	Pausable bool           `json:"pausable,omitempty"`
}

type TaskSnapshot struct {
	Label string `json:"label"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

func (b *progressBoard) Snapshot() Snapshot {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.active {
		return Snapshot{Paused: b.Paused()}
	}
	s := Snapshot{Active: true, Title: b.title, Files: b.unit == unitFiles, Done: b.done.Load(), Total: b.total, Paused: b.Paused(), Pausable: b.pausable.Load()}
	if b.unit == unitBytes {
		s.Speed = b.speedLocked()
	}
	for _, t := range b.tasks {
		s.Tasks = append(s.Tasks, TaskSnapshot{Label: t.label, Done: t.done.Load(), Total: t.total})
	}
	return s
}

func (b *progressBoard) Begin(title string, unit progressUnit, total, done int64) {
	b.End()
	b.mu.Lock()
	b.title, b.unit, b.total = title, unit, total
	b.done.Store(done)
	b.startDone = done
	b.started = time.Now()
	b.lastLine = b.started
	b.tasks, b.samples = nil, nil
	b.stop, b.stopped = make(chan struct{}), make(chan struct{})
	b.active = true
	b.pausable.Store(true)
	stop, stopped := b.stop, b.stopped
	b.mu.Unlock()
	go b.loop(stop, stopped)
}

func (b *progressBoard) loop(stop, stopped chan struct{}) {
	defer close(stopped)
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			b.mu.Lock()
			b.sampleLocked()
			b.mu.Unlock()
		}
	}
}

func (b *progressBoard) End() {
	b.mu.Lock()
	if !b.active {
		b.mu.Unlock()
		return
	}
	stop, stopped := b.stop, b.stopped
	b.mu.Unlock()
	close(stop)
	<-stopped
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tasks = nil
	log.Print(b.summaryLocked())
	b.active = false
}

func (b *progressBoard) Add(n int64) {
	b.Wait()
	b.done.Add(n)
}

func (b *progressBoard) Start(label string, total int64) *progressTask {
	t := &progressTask{label: label, total: total, board: b}
	b.mu.Lock()
	if b.active {
		b.tasks = append(b.tasks, t)
	}
	b.mu.Unlock()
	return t
}

func (b *progressBoard) finish(t *progressTask) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, x := range b.tasks {
		if x == t {
			b.tasks = append(b.tasks[:i], b.tasks[i+1:]...)
			return
		}
	}
}

func (b *progressBoard) sampleLocked() {
	if !b.active {
		return
	}
	now := time.Now()
	done := b.done.Load()
	b.samples = append(b.samples, progressSample{now, done})
	for len(b.samples) > 2 && now.Sub(b.samples[0].at) > 10*time.Second {
		b.samples = b.samples[1:]
	}
	if now.Sub(b.lastLine) >= 10*time.Second {
		log.Print(b.headlineLocked(done))
		b.lastLine = now
	}
}

func (b *progressBoard) headlineLocked(done int64) string {
	frac := fraction(done, b.total)
	parts := []string{fmt.Sprintf("%-12s", b.title), fmt.Sprintf("%5.1f%%", 100*frac), b.amount(done, b.total)}
	if b.unit == unitBytes {
		if speed := b.speedLocked(); speed > 0 {
			parts = append(parts, formatBytes(int64(speed))+"/s")
			if left := b.total - done; left > 0 {
				parts = append(parts, formatDuration(time.Duration(float64(left)/speed*float64(time.Second)))+" left")
			}
		}
	}
	return strings.Join(parts, "  ")
}

func (b *progressBoard) summaryLocked() string {
	done := b.done.Load()
	elapsed := time.Since(b.started)
	if done < b.total {
		return fmt.Sprintf("%-12s  stopped at %.1f%%, %s", b.title, 100*fraction(done, b.total), b.amount(done, b.total))
	}
	s := fmt.Sprintf("%-12s  done: %s in %s", b.title, b.amount(b.total, -1), formatDuration(elapsed))
	if moved := done - b.startDone; b.unit == unitBytes && moved > 0 && elapsed >= time.Second {
		s += fmt.Sprintf(", %s/s", formatBytes(int64(float64(moved)/elapsed.Seconds())))
	}
	return s
}

func (b *progressBoard) speedLocked() float64 {
	if len(b.samples) < 2 {
		return 0
	}
	first, last := b.samples[0], b.samples[len(b.samples)-1]
	dt := last.at.Sub(first.at).Seconds()
	if dt < 1 {
		return 0
	}
	return float64(last.done-first.done) / dt
}

func (b *progressBoard) amount(done, total int64) string {
	if b.unit == unitFiles {
		if total < 0 {
			return fmt.Sprintf("%d files", done)
		}
		return fmt.Sprintf("%d / %d files", done, total)
	}
	if total < 0 {
		return formatBytes(done)
	}
	return formatPair(done, total)
}

func fraction(done, total int64) float64 {
	if total <= 0 {
		return 1
	}
	f := float64(done) / float64(total)
	return min(max(f, 0), 1)
}

func formatBytes(n int64) string {
	switch {
	case n >= gib:
		return fmt.Sprintf("%.1f GiB", float64(n)/gib)
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KiB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

func FormatBytes(n int64) string { return formatBytes(n) }

func formatPair(done, total int64) string {
	switch {
	case total >= gib:
		return fmt.Sprintf("%.1f / %.1f GiB", float64(done)/gib, float64(total)/gib)
	case total >= 1<<20:
		return fmt.Sprintf("%.1f / %.1f MiB", float64(done)/(1<<20), float64(total)/(1<<20))
	case total >= 1<<10:
		return fmt.Sprintf("%.0f / %.0f KiB", float64(done)/(1<<10), float64(total)/(1<<10))
	}
	return fmt.Sprintf("%d / %d B", done, total)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h, m, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh%02dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
