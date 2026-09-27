package launcher

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type bdecode struct {
	data []byte
	at   int
}

func (d *bdecode) value(depth int) (any, error) {
	if depth > 20 || d.at >= len(d.data) {
		return nil, errors.New("invalid bencode")
	}
	switch d.data[d.at] {
	case 'i':
		d.at++
		end := bytes.IndexByte(d.data[d.at:], 'e')
		if end < 0 {
			return nil, errors.New("unterminated integer")
		}
		v, err := strconv.ParseInt(string(d.data[d.at:d.at+end]), 10, 64)
		d.at += end + 1
		return v, err
	case 'l':
		d.at++
		var list []any
		for d.at < len(d.data) && d.data[d.at] != 'e' {
			v, err := d.value(depth + 1)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		if d.at >= len(d.data) {
			return nil, errors.New("unterminated list")
		}
		d.at++
		return list, nil
	case 'd':
		d.at++
		dict := make(map[string]any)
		for d.at < len(d.data) && d.data[d.at] != 'e' {
			k, err := d.value(depth + 1)
			if err != nil {
				return nil, err
			}
			kb, ok := k.([]byte)
			if !ok {
				return nil, errors.New("non-string dictionary key")
			}
			v, err := d.value(depth + 1)
			if err != nil {
				return nil, err
			}
			dict[string(kb)] = v
		}
		if d.at >= len(d.data) {
			return nil, errors.New("unterminated dictionary")
		}
		d.at++
		return dict, nil
	default:
		colon := bytes.IndexByte(d.data[d.at:], ':')
		if colon < 0 {
			return nil, errors.New("missing byte-string length")
		}
		n, err := strconv.ParseInt(string(d.data[d.at:d.at+colon]), 10, 64)
		if err != nil || n < 0 || n > int64(len(d.data)) {
			return nil, errors.New("bad byte-string length")
		}
		d.at += colon + 1
		if n > int64(len(d.data)-d.at) {
			return nil, errors.New("truncated byte string")
		}
		v := d.data[d.at : d.at+int(n)]
		d.at += int(n)
		return v, nil
	}
}

func dict(v any) (map[string]any, error) {
	x, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("expected dictionary")
	}
	return x, nil
}
func bstr(v any) (string, error) {
	x, ok := v.([]byte)
	if !ok {
		return "", errors.New("expected byte string")
	}
	return string(x), nil
}
func bint(v any) (int64, error) {
	x, ok := v.(int64)
	if !ok {
		return 0, errors.New("expected integer")
	}
	return x, nil
}

type torrentFile struct {
	name    string
	size    int64
	padding bool
}
type torrentMeta struct {
	name      string
	webseed   string
	pieceSize int64
	hashes    []byte
	files     []torrentFile
}

func parseTorrent(data []byte) (torrentMeta, error) {
	var result torrentMeta
	d := bdecode{data: data}
	v, err := d.value(0)
	if err != nil {
		return result, err
	}
	if d.at != len(data) {
		return result, errors.New("trailing torrent bytes")
	}
	root, err := dict(v)
	if err != nil {
		return result, err
	}
	info, err := dict(root["info"])
	if err != nil {
		return result, err
	}
	result.name, err = bstr(info["name"])
	if err != nil {
		return result, err
	}
	if result.name == "" || strings.ContainsAny(result.name, `/\:`) || result.name == ".." {
		return result, errors.New("unsafe torrent name")
	}
	result.pieceSize, err = bint(info["piece length"])
	if err != nil || result.pieceSize <= 0 || result.pieceSize > 16<<20 {
		return result, errors.New("invalid piece length")
	}
	hashes, ok := info["pieces"].([]byte)
	if !ok || len(hashes)%sha1.Size != 0 {
		return result, errors.New("invalid piece hashes")
	}
	result.hashes = hashes
	var seeds []any
	switch x := root["url-list"].(type) {
	case []any:
		seeds = x
	case []byte:
		seeds = []any{x}
	}
	for _, s := range seeds {
		u, err := bstr(s)
		if err != nil {
			continue
		}
		parsed, err := url.Parse(u)
		if err == nil && parsed.Scheme == "http" && parsed.Hostname() == "pkg.dl.mail.ru" {
			result.webseed = strings.TrimRight(u, "/") + "/"
			break
		}
	}
	if result.webseed == "" {
		return result, errors.New("official HTTP webseed absent")
	}
	entries, ok := info["files"].([]any)
	if !ok || len(entries) == 0 || len(entries) > 10000 {
		return result, errors.New("invalid torrent file list")
	}
	var total int64
	for _, raw := range entries {
		item, err := dict(raw)
		if err != nil {
			return result, err
		}
		size, err := bint(item["length"])
		if err != nil || size < 0 {
			return result, errors.New("invalid file length")
		}
		parts, ok := item["path"].([]any)
		if !ok || len(parts) == 0 {
			return result, errors.New("missing file path")
		}
		var names []string
		for _, p := range parts {
			n, err := bstr(p)
			if err != nil || n == "" || n == "." || n == ".." || strings.ContainsAny(n, `\/:`) {
				return result, errors.New("unsafe torrent file path")
			}
			names = append(names, n)
		}
		name := path.Join(names...)
		result.files = append(result.files, torrentFile{name: name, size: size, padding: strings.HasPrefix(names[0], "_____padding_file_")})
		if total > 1<<50-size {
			return result, errors.New("torrent too large")
		}
		total += size
	}
	expected := (total + result.pieceSize - 1) / result.pieceSize
	if int64(len(hashes)/sha1.Size) != expected {
		return result, errors.New("torrent piece count mismatch")
	}
	return result, nil
}

func (t torrentMeta) download(client *http.Client, root string, jobs int) error {
	if err := t.fetchFiles(client, root, jobs); err != nil {
		return err
	}
	return t.verifyPieces(root)
}

func (t torrentMeta) fetchFiles(client *http.Client, root string, jobs int) error {
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	if jobs < 1 {
		jobs = 1
	}
	var total, done int64
	for _, file := range t.files {
		if file.padding {
			continue
		}
		total += file.size
		if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(file.name))); err == nil && st.Size() == file.size {
			done += file.size
		}
	}
	progress.Default.Begin("Downloading", progress.UnitBytes, total, done)
	defer progress.Default.End()
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	var once sync.Once
	var firstErr error
	for _, file := range t.files {
		if file.padding {
			continue
		}
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			dst := filepath.Join(root, filepath.FromSlash(file.name))
			u := t.webseed + url.PathEscape(t.name) + "/" + file.name
			if err := downloadOne(client, u, dst, file.name, file.size); err != nil {
				once.Do(func() { firstErr = fmt.Errorf("%s: %w", file.name, err) })
			}
		}()
	}
	wg.Wait()
	return firstErr
}

func downloadOne(client *http.Client, source, dst, label string, expected int64) error {
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
		req, err := http.NewRequest(http.MethodGet, source, nil)
		if err != nil {
			file.Close()
			return err
		}
		if pos > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", pos))
		}
		resp, err := client.Do(req)
		if err != nil {
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
			file.Close()
			return errors.New(status)
		}
		if _, err = file.Seek(pos, io.SeekStart); err != nil {
			resp.Body.Close()
			file.Close()
			return err
		}
		task.Set(pos)
		_, copyErr := io.Copy(io.MultiWriter(file, task), io.LimitReader(resp.Body, expected-pos+1))
		resp.Body.Close()
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

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func (t torrentMeta) verifyPieces(root string) error {
	bad, err := t.badFiles(root)
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		return fmt.Errorf("torrent SHA1 mismatch in %s", bad[0])
	}
	return nil
}

func (t torrentMeta) badFiles(root string) ([]string, error) {
	type span struct {
		name       string
		start, end int64
	}
	var spans []span
	var readers []io.Reader
	var opened []*os.File
	defer func() {
		for _, f := range opened {
			f.Close()
		}
	}()
	var total int64
	for _, item := range t.files {
		if !item.padding {
			spans = append(spans, span{item.name, total, total + item.size})
		}
		total += item.size
		if item.padding {
			readers = append(readers, io.LimitReader(zeroReader{}, item.size))
			continue
		}
		file, err := os.Open(filepath.Join(root, filepath.FromSlash(item.name)))
		if err != nil {
			return nil, err
		}
		opened = append(opened, file)
		readers = append(readers, io.LimitReader(file, item.size))
	}
	progress.Default.Begin("Verifying", progress.UnitBytes, total, 0)
	defer progress.Default.End()
	stream := io.MultiReader(readers...)
	badSet := map[string]bool{}
	var bad []string
	for i := 0; i < len(t.hashes)/sha1.Size; i++ {
		h := sha1.New()
		n := t.pieceSize
		if total < n {
			n = total
		}
		if _, err := io.CopyN(h, stream, n); err != nil {
			return nil, fmt.Errorf("piece %d: %w", i, err)
		}
		if !bytes.Equal(h.Sum(nil), t.hashes[i*sha1.Size:(i+1)*sha1.Size]) {
			start := int64(i) * t.pieceSize
			for _, s := range spans {
				if s.start < start+n && start < s.end && !badSet[s.name] {
					badSet[s.name] = true
					bad = append(bad, s.name)
				}
			}
		}
		total -= n
		progress.Default.Add(n)
	}
	if total != 0 {
		return nil, errors.New("torrent verification ended early")
	}
	return bad, nil
}
