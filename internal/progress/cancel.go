package progress

import (
	"context"
	"errors"
	"sync"
)

// ErrCancelled is returned by the board's readers and writers once the person
// cancels the running operation.
var ErrCancelled = errors.New("cancelled")

type cancelGate struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

func (g *cancelGate) arm() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cancel != nil {
		g.cancel()
	}
	g.ctx, g.cancel = context.WithCancel(context.Background())
}

func (g *cancelGate) context() context.Context {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.ctx == nil {
		return context.Background()
	}
	return g.ctx
}

// Arm starts a new operation that can be cancelled and forgets an earlier
// cancel.
func (b *Board) Arm() { b.root().abort.arm() }

// Cancel stops the armed operation: downloads stop, a pause ends, and every
// read or write through the board fails with ErrCancelled.
func (b *Board) Cancel() bool {
	r := b.root()
	r.abort.mu.Lock()
	cancel := r.abort.cancel
	done := r.abort.ctx != nil && r.abort.ctx.Err() != nil
	r.abort.mu.Unlock()
	if cancel == nil || done {
		return false
	}
	cancel()
	r.gate.unpause()
	return true
}

// Context is done once the armed operation is cancelled.
func (b *Board) Context() context.Context { return b.root().abort.context() }

func (b *Board) Cancelled() bool { return b.Context().Err() != nil }

func (b *Board) Err() error {
	if b.Cancelled() {
		return ErrCancelled
	}
	return nil
}

func (b *Board) root() *Board {
	if b.parent != nil {
		return b.parent.root()
	}
	return b
}
