package download

import (
	"context"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

var stallTimeout = time.Minute

var Transport = func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConnsPerHost = 16
	return t
}()

type stallReader struct {
	r     io.Reader
	timer *time.Timer
}

func (s stallReader) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	if n > 0 {
		s.timer.Reset(stallTimeout)
	}
	return n, err
}

type Request struct {
	Client   *http.Client
	URL      string
	Dst      string
	Label    string
	Size     int64
	NewHash  func() hash.Hash
	Progress *progress.Board
}

func File(client *http.Client, source, dst, label string, expected int64) error {
	_, err := Fetch(context.Background(), Request{Client: client, URL: source, Dst: dst, Label: label, Size: expected})
	return err
}

func Fetch(ctx context.Context, r Request) ([]byte, error) {
	if info, err := os.Stat(r.Dst); err == nil && info.Size() == r.Size {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(r.Dst), 0755); err != nil {
		return nil, err
	}
	board := r.Progress
	if board == nil {
		board = progress.Default
	}
	task := board.Start(r.Label, r.Size)
	defer task.Done()
	partial := r.Dst + ".part"
	for attempt := 0; attempt < 8; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		board.Wait()
		pauses := board.Pauses()
		sum, done, err := fetchOnce(ctx, r, partial, task)
		if done || err != nil {
			return sum, err
		}
		if ctx.Err() == nil && board.Pauses() != pauses {
			attempt--
			continue
		}
		sleep(ctx, time.Second)
	}
	return nil, errors.New("download did not complete after retries")
}

func fetchOnce(ctx context.Context, r Request, partial string, task *progress.Task) ([]byte, bool, error) {
	file, err := os.OpenFile(partial, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	pos := info.Size()
	if pos > r.Size {
		file.Truncate(0)
		pos = 0
	}
	if pos == r.Size {
		h, err := hashPrefix(file, r.NewHash, pos)
		if err != nil {
			return nil, false, err
		}
		file.Close()
		return sumOf(h), true, os.Rename(partial, r.Dst)
	}
	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stall := time.AfterFunc(stallTimeout, cancel)
	defer stall.Stop()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, r.URL, nil)
	if err != nil {
		return nil, false, err
	}
	if pos > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", pos))
	}
	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, false, nil
	}
	defer resp.Body.Close()
	if pos > 0 && resp.StatusCode == http.StatusOK {
		file.Truncate(0)
		pos = 0
	}
	if (pos > 0 && resp.StatusCode != http.StatusPartialContent) || (pos == 0 && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent) {
		return nil, false, errors.New(resp.Status)
	}
	if _, err = file.Seek(pos, io.SeekStart); err != nil {
		return nil, false, err
	}
	h, err := hashPrefix(file, r.NewHash, pos)
	if err != nil {
		return nil, false, err
	}
	task.Set(pos)
	out := io.MultiWriter(file, task)
	if h != nil {
		out = io.MultiWriter(file, task, h)
	}
	if _, err := io.Copy(out, io.LimitReader(stallReader{resp.Body, stall}, r.Size-pos+1)); err != nil {
		return nil, false, nil
	}
	info, err = file.Stat()
	if err != nil {
		return nil, false, err
	}
	if info.Size() > r.Size {
		return nil, false, errors.New("HTTP response exceeds expected size")
	}
	if info.Size() < r.Size {
		return nil, false, nil
	}
	file.Close()
	return sumOf(h), true, os.Rename(partial, r.Dst)
}

func hashPrefix(file *os.File, newHash func() hash.Hash, n int64) (hash.Hash, error) {
	if newHash == nil {
		return nil, nil
	}
	h := newHash()
	if n > 0 {
		if _, err := io.Copy(h, io.NewSectionReader(file, 0, n)); err != nil {
			return nil, err
		}
	}
	return h, nil
}

func sumOf(h hash.Hash) []byte {
	if h == nil {
		return nil
	}
	return h.Sum(nil)
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
