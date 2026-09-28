package gamefiles

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/catalog"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/torrent"
)

type vkDistrib struct {
	info       patchInfo
	meta       torrent.Meta
	manifest   Manifest
	compressed []byte
}

func fetchVKDistrib(build int) (vkDistrib, error) {
	client := &http.Client{Timeout: 90 * time.Second}
	info, err := catalog.LatestDistrib(client)
	if err != nil {
		return vkDistrib{}, err
	}
	if build != 0 && info.Destination != build {
		return vkDistrib{}, fmt.Errorf("the official full client is build %d, but the installed game is build %d; update it first", info.Destination, build)
	}
	meta, err := fetchTorrent(client, info.TorrentURL, info.TorrentSHA1)
	if err != nil {
		return vkDistrib{}, err
	}
	manifest, compressed, err := fetchFullClientManifest(client, meta, info)
	if err != nil {
		return vkDistrib{}, err
	}
	return vkDistrib{info: info, meta: meta, manifest: manifest, compressed: compressed}, nil
}

func fetchTorrent(client *http.Client, link, sha1 string) (torrent.Meta, error) {
	data, err := catalog.Fetch(client, link, 16<<20)
	if err != nil {
		return torrent.Meta{}, err
	}
	if err := catalog.VerifyHexDigest(data, sha1, "sha1"); err != nil {
		return torrent.Meta{}, fmt.Errorf("torrent: %w", err)
	}
	return torrent.Parse(data)
}

func fetchFullClientManifest(client *http.Client, meta torrent.Meta, distrib patchInfo) (Manifest, []byte, error) {
	var manifestName string
	for _, f := range meta.Files {
		if strings.EqualFold(f.Name, "manifest.xml.gz") {
			manifestName = f.Name
			break
		}
	}
	if manifestName == "" {
		return Manifest{}, nil, errors.New("full client torrent has no file manifest")
	}
	compressed, err := catalog.Fetch(client, meta.Webseed+url.PathEscape(meta.Name)+"/"+manifestName, 16<<20)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("file manifest: %w", err)
	}
	manifest, err := parsePatchManifest(compressed, distrib)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("file manifest: %w", err)
	}
	return manifest, compressed, nil
}

func (d vkDistrib) files(only []string) (fileSet, error) {
	listed := map[string]torrent.File{}
	for _, f := range d.meta.Files {
		if !f.Padding {
			listed[normalizedClientName(f.Name)] = f
		}
	}
	var wanted map[string]bool
	if only != nil {
		wanted = map[string]bool{}
		for _, name := range only {
			wanted[normalizedClientName(name)] = true
		}
	}
	set := fileSet{NewHash: md5.New, Algo: "MD5"}
	for _, f := range d.manifest.NonCompressed.Files {
		key := normalizedClientName(f.Name)
		if wanted != nil && !wanted[key] {
			continue
		}
		delete(wanted, key)
		entry, ok := listed[key]
		if !ok || entry.Size != f.Size {
			return fileSet{}, fmt.Errorf("%s does not match the official full client", f.Name)
		}
		if digest, err := hex.DecodeString(f.MD5); err != nil || len(digest) != md5.Size {
			return fileSet{}, fmt.Errorf("%s has no valid MD5 in the manifest", f.Name)
		}
		set.Files = append(set.Files, remoteFile{
			Path:     f.Name,
			Size:     f.Size,
			Hash:     f.MD5,
			URL:      d.meta.Webseed + url.PathEscape(d.meta.Name) + "/" + entry.Name,
			Modified: f.Modified,
		})
	}
	for name := range wanted {
		return fileSet{}, fmt.Errorf("%s does not match the official full client", name)
	}
	if len(set.Files) == 0 && only == nil {
		return fileSet{}, errors.New("empty VK Play client manifest")
	}
	return set, nil
}

func normalizedClientName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, `\`, "/"))
}
