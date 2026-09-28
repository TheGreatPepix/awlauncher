package vkplay

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"

	"github.com/TheGreatPepix/awlauncher/internal/httpx"
)

const patchBase = "http://static.dl.mail.ru/torrents/patches/armoredwarfare_hd"

type Index struct {
	XMLName  xml.Name    `xml:"Patches"`
	Name     string      `xml:"Name,attr"`
	Patches  []PatchInfo `xml:"PatchItem"`
	Distribs []PatchInfo `xml:"PureClient"`
}

type PatchInfo struct {
	Source        int    `xml:"SrcBuildId,attr"`
	Destination   int    `xml:"DestBuildId,attr"`
	TorrentURL    string `xml:"TorrentUrl,attr"`
	TorrentSHA1   string `xml:"TorrentSign,attr"`
	ManifestSHA   string `xml:"ManifestSign,attr"`
	ModifiedUnix  int64  `xml:"LastModified,attr"`
	InstalledSize int64  `xml:"InstalledTotalSize,attr"`
	DownloadSize  int64  `xml:"PatchSize,attr"`
}

func ReadXML(data []byte, value any) error {
	return xml.Unmarshal(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), value)
}

type head struct {
	Build int    `xml:"BuildId,attr"`
	Name  string `xml:"Name,attr"`
}

func latestBuild(client *http.Client) (head, error) {
	var h head
	head, err := httpx.Get(client, patchBase+"_head.xml?gid=0.11321", 1<<20)
	if err != nil {
		return h, err
	}
	if err := ReadXML(head, &h); err != nil {
		return h, err
	}
	if h.Name != "armoredwarfare_hd" || h.Build <= 0 {
		return h, errors.New("unexpected patch head")
	}
	return h, nil
}

func fetchCatalog(client *http.Client, h head) (Index, error) {
	var index Index
	data, err := httpx.Get(client, patchBase+"0.xml?gid=0.11321", 8<<20)
	if err != nil {
		return index, err
	}
	if err := ReadXML(data, &index); err != nil {
		return index, err
	}
	if index.Name != h.Name {
		return index, errors.New("patch catalog belongs to another game")
	}
	return index, nil
}

func LatestDistrib(client *http.Client) (PatchInfo, error) {
	h, err := latestBuild(client)
	if err != nil {
		return PatchInfo{}, err
	}
	index, err := fetchCatalog(client, h)
	if err != nil {
		return PatchInfo{}, err
	}
	for _, d := range index.Distribs {
		if d.Source == 0 && d.Destination == h.Build && d.TorrentURL != "" {
			return d, nil
		}
	}
	return PatchInfo{}, fmt.Errorf("the catalog has no full client for build %d", h.Build)
}

func LatestPatches(client *http.Client, installed int) ([]PatchInfo, int, error) {
	h, err := latestBuild(client)
	if err != nil {
		return nil, 0, err
	}
	if installed >= h.Build {
		return nil, h.Build, nil
	}
	index, err := fetchCatalog(client, h)
	if err != nil {
		return nil, 0, err
	}
	distance := map[int]int64{installed: 0}
	predecessor := map[int]PatchInfo{}
	for changed := true; changed; {
		changed = false
		for _, p := range index.Patches {
			d, ok := distance[p.Source]
			if !ok || p.Destination <= p.Source || p.Destination > h.Build {
				continue
			}
			size := p.DownloadSize
			if size <= 0 {
				size = 1 << 60
			}
			old, seen := distance[p.Destination]
			if !seen || d+size < old {
				distance[p.Destination] = d + size
				predecessor[p.Destination] = p
				changed = true
			}
		}
	}
	if _, ok := predecessor[h.Build]; !ok {
		return nil, h.Build, fmt.Errorf("no patch path from build %d to %d", installed, h.Build)
	}
	var path []PatchInfo
	for build := h.Build; build != installed; {
		p, ok := predecessor[build]
		if !ok {
			return nil, h.Build, errors.New("broken patch path")
		}
		path = append(path, p)
		build = p.Source
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path, h.Build, nil
}
