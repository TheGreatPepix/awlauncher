package launcher

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func repairClientFiles(g gameInstall, damaged []inventoryFile) error {
	if err := ensureGameClosed(); err != nil {
		return err
	}
	if err := markFXDirty(g.Root); err != nil {
		return err
	}
	client := &http.Client{Timeout: 90 * time.Second}
	distrib, err := latestDistrib(client)
	if err != nil {
		return err
	}
	if distrib.Destination != g.Build {
		return fmt.Errorf("the official full client is build %d, but the installed game is build %d; update it first", distrib.Destination, g.Build)
	}
	torrent, err := fetch(client, distrib.TorrentURL, 16<<20)
	if err != nil {
		return err
	}
	if err := verifyHexDigest(torrent, distrib.TorrentSHA1, "sha1"); err != nil {
		return fmt.Errorf("torrent: %w", err)
	}
	meta, err := parseTorrent(torrent)
	if err != nil {
		return err
	}
	manifest, _, err := fetchFullClientManifest(client, meta, distrib)
	if err != nil {
		return err
	}
	if err := repairListedFiles(&http.Client{Timeout: 2 * time.Hour}, g.Root, meta, manifest, damaged); err != nil {
		return err
	}
	return markFXDirty(g.Root)
}

func fetchFullClientManifest(client *http.Client, meta torrentMeta, distrib patchInfo) (patchManifest, []byte, error) {
	var manifestName string
	for _, f := range meta.files {
		if strings.EqualFold(f.name, "manifest.xml.gz") {
			manifestName = f.name
			break
		}
	}
	if manifestName == "" {
		return patchManifest{}, nil, errors.New("full client torrent has no file manifest")
	}
	compressed, err := fetch(client, meta.webseed+url.PathEscape(meta.name)+"/"+manifestName, 16<<20)
	if err != nil {
		return patchManifest{}, nil, fmt.Errorf("file manifest: %w", err)
	}
	manifest, err := parsePatchManifest(compressed, distrib)
	if err != nil {
		return patchManifest{}, nil, fmt.Errorf("file manifest: %w", err)
	}
	return manifest, compressed, nil
}

func normalizedClientName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, `\`, "/"))
}

func repairListedFiles(client *http.Client, root string, meta torrentMeta, manifest patchManifest, damaged []inventoryFile) error {
	listed := map[string]torrentFile{}
	for _, f := range meta.files {
		if !f.padding {
			listed[normalizedClientName(f.name)] = f
		}
	}
	files := map[string]manifestFile{}
	for _, f := range manifest.NonCompressed.Files {
		files[normalizedClientName(f.Name)] = f
	}
	type job struct {
		file manifestFile
		url  string
	}
	var jobs []job
	var total int64
	for _, missing := range damaged {
		name := normalizedClientName(missing.Name)
		file, ok := files[name]
		entry, inTorrent := listed[name]
		if !ok || !inTorrent || file.Size != entry.size || file.Size != missing.Size {
			return fmt.Errorf("%s does not match the official full client", missing.Name)
		}
		if _, err := safeGamePath(root, file.Name); err != nil {
			return err
		}
		digest, err := hex.DecodeString(file.MD5)
		if err != nil || len(digest) != md5.Size {
			return fmt.Errorf("%s has no valid MD5 in the manifest", file.Name)
		}
		jobs = append(jobs, job{file: file, url: meta.webseed + url.PathEscape(meta.name) + "/" + entry.name})
		total += file.Size
	}
	if len(jobs) == 0 {
		return nil
	}
	if free, err := diskFree(root); err == nil && free < total {
		return fmt.Errorf("not enough free space: %s needed", progress.FormatBytes(total))
	}
	fmt.Printf("Downloading %d files (%s)...\n", len(jobs), progress.FormatBytes(total))
	progress.Default.Begin("Downloading", progress.UnitBytes, total, 0)
	err := parallel(jobs, 3, func(j job) error {
		dst, _ := safeGamePath(root, j.file.Name)
		for attempt := 0; attempt < 2; attempt++ {
			if err := downloadOne(client, j.url, dst, j.file.Name, j.file.Size); err != nil {
				return fmt.Errorf("%s: %w", j.file.Name, err)
			}
			if err := verifyRepairedFile(dst, j.file); err == nil {
				return nil
			}
			if err := os.Remove(dst); err != nil {
				return err
			}
		}
		return fmt.Errorf("%s: MD5 mismatch after download", j.file.Name)
	})
	progress.Default.End()
	if err != nil {
		return fmt.Errorf("%w (run the launcher again to resume)", err)
	}
	fmt.Println("Client repair complete; downloaded files match the official manifest.")
	return nil
}

func verifyRepairedFile(path string, item manifestFile) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() != item.Size {
		return errors.New("wrong file size")
	}
	h := md5.New()
	if _, err := io.Copy(h, progress.Default.Reader(f)); err != nil {
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), item.MD5) {
		return errors.New("MD5 mismatch")
	}
	if item.Modified > 0 {
		mtime := time.Unix(item.Modified, 0)
		_ = os.Chtimes(path, mtime, mtime)
	}
	return nil
}
