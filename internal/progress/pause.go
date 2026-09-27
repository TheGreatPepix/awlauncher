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

func (b *progressBoard) Pause() bool { return b.gate.pause() }

func (b *progressBoard) Resume() bool { return b.gate.unpause() }

func (b *progressBoard) Paused() bool { return b.gate.paused.Load() }

func (b *progressBoard) Pauses() int64 { return b.gate.count.Load() }

func (b *progressBoard) NoPause() { b.pausable.Store(false) }

func (b *progressBoard) Wait() {
	if b.pausable.Load() {
		b.gate.wait()
	}
}

func (b *progressBoard) Reader(r io.Reader) io.Reader { return pausedReader{r, b} }

type pausedReader struct {
	r io.Reader
	b *progressBoard
}

func (p pausedReader) Read(buf []byte) (int, error) {
	p.b.Wait()
	return p.r.Read(buf)
}
