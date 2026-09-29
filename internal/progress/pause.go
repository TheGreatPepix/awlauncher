package progress

import (
	"io"
	"sync"
	"sync/atomic"
)

type pauseGate struct {
	paused atomic.Bool
	count  atomic.Int64
	mu     sync.Mutex
	resume chan struct{}
}

func (g *pauseGate) pause() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.paused.Load() {
		return false
	}
	g.resume = make(chan struct{})
	g.count.Add(1)
	g.paused.Store(true)
	return true
}

func (g *pauseGate) unpause() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.paused.Load() {
		return false
	}
	g.paused.Store(false)
	close(g.resume)
	return true
}

func (g *pauseGate) wait() {
	if !g.paused.Load() {
		return
	}
	g.mu.Lock()
	ch := g.resume
	paused := g.paused.Load()
	g.mu.Unlock()
	if paused {
		<-ch
	}
}

func (b *Board) Pause() bool { return b.gate.pause() }

func (b *Board) Resume() bool { return b.gate.unpause() }

func (b *Board) Paused() bool { return b.gate.paused.Load() }

func (b *Board) Pauses() int64 {
	if b.parent != nil {
		return b.parent.Pauses()
	}
	return b.gate.count.Load()
}

func (b *Board) NoPause() { b.pausable.Store(false) }

func (b *Board) Wait() {
	if b.parent != nil {
		b.parent.Wait()
		return
	}
	if b.pausable.Load() {
		b.gate.wait()
	}
}

func (b *Board) Quiet() *Board { return &Board{parent: b} }

func (b *Board) Reader(r io.Reader) io.Reader { return pausedReader{r, b} }

type pausedReader struct {
	r io.Reader
	b *Board
}

func (p pausedReader) Read(buf []byte) (int, error) {
	p.b.Wait()
	return p.r.Read(buf)
}
