package gamefiles

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type failingWriter struct{ after int }

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.after <= 0 {
		return 0, errors.New("disk full")
	}
	w.after--
	return len(p), nil
}

func TestCopyAhead(t *testing.T) {
	data := stageFixtureFile(1, 3*aheadChunk+12345)
	var out bytes.Buffer
	n, err := copyAhead(&out, bytes.NewReader(data))
	if err != nil || n != int64(len(data)) || !bytes.Equal(out.Bytes(), data) {
		t.Fatalf("copied %d, %v", n, err)
	}
	if _, err := copyAhead(&failingWriter{after: 1}, bytes.NewReader(data)); err == nil || err.Error() != "disk full" {
		t.Fatalf("write error: %v", err)
	}
	broken := io.MultiReader(bytes.NewReader(data[:aheadChunk+1]), iotestErrReader{})
	if _, err := copyAhead(io.Discard, broken); err == nil || err.Error() != "read failed" {
		t.Fatalf("read error: %v", err)
	}
}

func TestPipeAhead(t *testing.T) {
	data := stageFixtureFile(2, 2*aheadChunk+7)
	var out bytes.Buffer
	if err := pipeAhead(&out, func(w io.Writer) error { _, err := w.Write(data); return err }); err != nil || !bytes.Equal(out.Bytes(), data) {
		t.Fatalf("pipe: %v", err)
	}
	if err := pipeAhead(io.Discard, func(io.Writer) error { return errors.New("bad delta") }); err == nil || err.Error() != "bad delta" {
		t.Fatalf("producer error: %v", err)
	}
	if err := pipeAhead(&failingWriter{}, func(w io.Writer) error { _, err := w.Write(data); return err }); err == nil || err.Error() != "disk full" {
		t.Fatalf("writer error: %v", err)
	}
}

type iotestErrReader struct{}

func (iotestErrReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
