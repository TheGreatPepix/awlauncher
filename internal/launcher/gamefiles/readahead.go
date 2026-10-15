package gamefiles

import (
	"errors"
	"io"
	"sync"
)

const (
	aheadChunk = 2 << 20
	aheadDepth = 3
)

var aheadBuffers = sync.Pool{New: func() any {
	buf := make([]byte, aheadChunk)
	return &buf
}}

// copyAhead copies src to dst like io.Copy, but reads the next chunks while dst
// is still busy with the current one, so reading, decoding and hashing overlap.
func copyAhead(dst io.Writer, src io.Reader) (int64, error) {
	type chunk struct {
		buf *[]byte
		n   int
	}
	full := make(chan chunk, aheadDepth)
	stop := make(chan struct{})
	var readErr error
	go func() {
		defer close(full)
		for {
			buf := aheadBuffers.Get().(*[]byte)
			n, err := io.ReadFull(src, *buf)
			if n > 0 {
				select {
				case full <- chunk{buf, n}:
				case <-stop:
					aheadBuffers.Put(buf)
					return
				}
			} else {
				aheadBuffers.Put(buf)
			}
			if err != nil {
				if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
					readErr = err
				}
				return
			}
		}
	}()
	var written int64
	var writeErr error
	for c := range full {
		if writeErr == nil {
			var n int
			n, writeErr = dst.Write((*c.buf)[:c.n])
			written += int64(n)
			if writeErr == nil && n != c.n {
				writeErr = io.ErrShortWrite
			}
			if writeErr != nil {
				close(stop)
			}
		}
		aheadBuffers.Put(c.buf)
	}
	if writeErr != nil {
		return written, writeErr
	}
	return written, readErr
}

// pipeAhead runs produce in its own goroutine and copies what it writes to dst
// with copyAhead, so the producer never waits for dst.
func pipeAhead(dst io.Writer, produce func(io.Writer) error) error {
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := produce(pw)
		pw.CloseWithError(err)
		done <- err
	}()
	_, copyErr := copyAhead(dst, pr)
	pr.CloseWithError(errPipeClosed)
	produceErr := <-done
	if produceErr != nil && !errors.Is(produceErr, errPipeClosed) {
		return produceErr
	}
	return copyErr
}

var errPipeClosed = errors.New("output closed")
