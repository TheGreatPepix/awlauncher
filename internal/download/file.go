package download

import (
	"context"
	"errors"
	"fmt"
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

func File(client *http.Client, source, dst, label string, expected int64) error {
	if info, err := os.Stat(dst); err == nil && info.Size() == expected {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	task := progress.Default.Start(label, expected)
	defer task.Done()
	partial := dst + ".part"
	for attempt := 0; attempt < 8; attempt++ {
		progress.Default.Wait()
		pauses := progress.Default.Pauses()
		file, err := os.OpenFile(partial, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return err
		}
		info, err := file.Stat()
		if err != nil {
			file.Close()
			return err
		}
		pos := info.Size()
		if pos > expected {
			file.Truncate(0)
			pos = 0
		}
		if pos == expected {
			file.Close()
			return os.Rename(partial, dst)
		}
		ctx, cancel := context.WithCancel(context.Background())
		stall := time.AfterFunc(stallTimeout, cancel)
		stop := func() {
			stall.Stop()
			cancel()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			stop()
			file.Close()
			return err
		}
		if pos > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", pos))
		}
		resp, err := client.Do(req)
		if err != nil {
			stop()
			file.Close()
			time.Sleep(time.Second)
			continue
		}
		if pos > 0 && resp.StatusCode == http.StatusOK {
			file.Truncate(0)
			pos = 0
		}
		if (pos > 0 && resp.StatusCode != http.StatusPartialContent) || (pos == 0 && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent) {
			status := resp.Status
			resp.Body.Close()
			stop()
			file.Close()
			return errors.New(status)
		}
		if _, err = file.Seek(pos, io.SeekStart); err != nil {
			resp.Body.Close()
			stop()
			file.Close()
			return err
		}
		task.Set(pos)
		_, copyErr := io.Copy(io.MultiWriter(file, task), io.LimitReader(stallReader{resp.Body, stall}, expected-pos+1))
		resp.Body.Close()
		stop()
		file.Close()
		if copyErr != nil {
			if progress.Default.Pauses() != pauses {
				attempt--
				continue
			}
			time.Sleep(time.Second)
			continue
		}
		info, err = os.Stat(partial)
		if err != nil {
			return err
		}
		if info.Size() == expected {
			return os.Rename(partial, dst)
		}
		if info.Size() > expected {
			return errors.New("HTTP response exceeds expected size")
		}
		time.Sleep(time.Second)
	}
	return errors.New("download did not complete after retries")
}
