package torrent

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/download"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
	"github.com/TheGreatPepix/awlauncher/internal/workers"
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

type File struct {
	Name    string
	Size    int64
	Padding bool
}
type Meta struct {
	Name      string
	Webseed   string
	PieceSize int64
	Hashes    []byte
	Files     []File
}

func Parse(data []byte) (Meta, error) {
	var result Meta
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
	result.Name, err = bstr(info["name"])
	if err != nil {
		return result, err
	}
	if result.Name == "" || strings.ContainsAny(result.Name, `/\:`) || result.Name == ".." {
		return result, errors.New("unsafe torrent name")
	}
	result.PieceSize, err = bint(info["piece length"])
	if err != nil || result.PieceSize <= 0 || result.PieceSize > 16<<20 {
		return result, errors.New("invalid piece length")
	}
	hashes, ok := info["pieces"].([]byte)
	if !ok || len(hashes)%sha1.Size != 0 {
		return result, errors.New("invalid piece hashes")
	}
	result.Hashes = hashes
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
			result.Webseed = strings.TrimRight(u, "/") + "/"
			break
		}
	}
	if result.Webseed == "" {
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
		result.Files = append(result.Files, File{Name: name, Size: size, Padding: strings.HasPrefix(names[0], "_____padding_file_")})
		if total > 1<<50-size {
			return result, errors.New("torrent too large")
		}
		total += size
	}
	expected := (total + result.PieceSize - 1) / result.PieceSize
	if int64(len(hashes)/sha1.Size) != expected {
		return result, errors.New("torrent piece count mismatch")
	}
	return result, nil
}

func (t Meta) Download(client *http.Client, root string, jobs int) error {
	if err := t.fetchFiles(client, root, jobs); err != nil {
		return err
	}
	return t.verifyPieces(root)
}

func (t Meta) fetchFiles(client *http.Client, root string, jobs int) error {
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	var files []File
	var total, done int64
	for _, file := range t.Files {
		if file.Padding {
			continue
		}
		files = append(files, file)
		total += file.Size
		if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(file.Name))); err == nil && st.Size() == file.Size {
			done += file.Size
		}
	}
	progress.Default.Begin("Downloading", progress.UnitBytes, total, done)
	defer progress.Default.End()
	return workers.Each(files, jobs, func(file File) error {
		dst := filepath.Join(root, filepath.FromSlash(file.Name))
		u := t.Webseed + url.PathEscape(t.Name) + "/" + file.Name
		if err := download.File(client, u, dst, file.Name, file.Size); err != nil {
			return fmt.Errorf("%s: %w", file.Name, err)
		}
		return nil
	})
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func (t Meta) verifyPieces(root string) error {
	bad, err := t.badFiles(root)
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		return fmt.Errorf("torrent SHA1 mismatch in %s", bad[0])
	}
	return nil
}

func (t Meta) badFiles(root string) ([]string, error) {
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
	for _, item := range t.Files {
		if !item.Padding {
			spans = append(spans, span{item.Name, total, total + item.Size})
		}
		total += item.Size
		if item.Padding {
			readers = append(readers, io.LimitReader(zeroReader{}, item.Size))
			continue
		}
		file, err := os.Open(filepath.Join(root, filepath.FromSlash(item.Name)))
		if err != nil {
			return nil, err
		}
		opened = append(opened, file)
		readers = append(readers, io.LimitReader(file, item.Size))
	}
	progress.Default.Begin("Verifying", progress.UnitBytes, total, 0)
	defer progress.Default.End()
	stream := io.MultiReader(readers...)
	badSet := map[string]bool{}
	var bad []string
	for i := 0; i < len(t.Hashes)/sha1.Size; i++ {
		h := sha1.New()
		n := t.PieceSize
		if total < n {
			n = total
		}
		if _, err := io.CopyN(h, stream, n); err != nil {
			return nil, fmt.Errorf("piece %d: %w", i, err)
		}
		if !bytes.Equal(h.Sum(nil), t.Hashes[i*sha1.Size:(i+1)*sha1.Size]) {
			start := int64(i) * t.PieceSize
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
