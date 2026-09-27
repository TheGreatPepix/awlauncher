package progress

import (
	"fmt"
	"os"
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
	live      bool
	active    bool
	pausable  atomic.Bool
	gate      pauseGate
	title     string
	unit      progressUnit
	total     int64
	done      atomic.Int64
	startDone int64
	started   time.Time
	lastPlain time.Time
	tasks     []*progressTask
	samples   []progressSample
	drawn     int
	stop      chan struct{}
	stopped   chan struct{}
}

var ui = &progressBoard{live: enableVT()}

var Default = ui

func UsePlainOutput() {
	ui.mu.Lock()
	ui.live = false
	ui.mu.Unlock()
}

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
	b.lastPlain = b.started
	b.tasks, b.samples, b.drawn = nil, nil, 0
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
			b.renderLocked()
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
	line := b.summaryLocked()
	if b.live {
		b.writeLocked([]string{line})
		b.drawn = 0
	} else {
		fmt.Println(line)
	}
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

func (b *progressBoard) Log(format string, args ...any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.live && b.drawn > 0 {
		fmt.Fprintf(os.Stdout, "\x1b[%dA\r\x1b[J", b.drawn)
		b.drawn = 0
	}
	fmt.Printf(format+"\n", args...)
	if b.live && b.active {
		b.renderLocked()
	}
}

func (b *progressBoard) renderLocked() {
	if !b.active {
		return
	}
	now := time.Now()
	done := b.done.Load()
	b.samples = append(b.samples, progressSample{now, done})
	for len(b.samples) > 2 && now.Sub(b.samples[0].at) > 10*time.Second {
		b.samples = b.samples[1:]
	}
	if !b.live {
		if now.Sub(b.lastPlain) >= 10*time.Second {
			fmt.Println(b.headlineLocked(done, false))
			b.lastPlain = now
		}
		return
	}
	lines := []string{b.headlineLocked(done, true)}
	for _, t := range b.tasks {
		lines = append(lines, taskLine(t))
	}
	b.writeLocked(lines)
}

func (b *progressBoard) writeLocked(lines []string) {
	width := consoleWidth() - 1
	var s strings.Builder
	if b.drawn > 0 {
		fmt.Fprintf(&s, "\x1b[%dA", b.drawn)
	}
	s.WriteString("\r")
	for _, l := range lines {
		s.WriteString("\x1b[2K")
		s.WriteString(clipRight(l, width))
		s.WriteString("\n")
	}
	s.WriteString("\x1b[J")
	os.Stdout.WriteString(s.String())
	b.drawn = len(lines)
}

func (b *progressBoard) headlineLocked(done int64, bar bool) string {
	frac := fraction(done, b.total)
	parts := []string{fmt.Sprintf("%-12s", b.title)}
	if bar {
		parts = append(parts, drawBar(frac, 30))
	}
	parts = append(parts, fmt.Sprintf("%5.1f%%", 100*frac), b.amount(done, b.total))
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

func taskLine(t *progressTask) string {
	label := fmt.Sprintf("  %-32s", clipLeft(t.label, 32))
	if t.total <= 0 {
		return label
	}
	done := t.done.Load()
	frac := fraction(done, t.total)
	return fmt.Sprintf("%s  %s %5.1f%%  %s", label, drawBar(frac, 12), 100*frac, formatPair(done, t.total))
}

func fraction(done, total int64) float64 {
	if total <= 0 {
		return 1
	}
	f := float64(done) / float64(total)
	return min(max(f, 0), 1)
}

func drawBar(frac float64, width int) string {
	filled := int(frac * float64(width))
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
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

func clipRight(s string, width int) string {
	r := []rune(s)
	if width < 1 || len(r) <= width {
		return s
	}
	return string(r[:width-1]) + "…"
}

func clipLeft(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	return "…" + string(r[len(r)-width+1:])
}
