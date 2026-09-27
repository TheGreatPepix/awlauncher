package launcher

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const patchBase = "http://static.dl.mail.ru/torrents/patches/armoredwarfare_hd"

type patchIndex struct {
	XMLName  xml.Name    `xml:"Patches"`
	Name     string      `xml:"Name,attr"`
	Patches  []patchInfo `xml:"PatchItem"`
	Distribs []patchInfo `xml:"PureClient"`
}

type patchInfo struct {
	Source        int    `xml:"SrcBuildId,attr"`
	Destination   int    `xml:"DestBuildId,attr"`
	TorrentURL    string `xml:"TorrentUrl,attr"`
	TorrentSHA1   string `xml:"TorrentSign,attr"`
	ManifestSHA   string `xml:"ManifestSign,attr"`
	ModifiedUnix  int64  `xml:"LastModified,attr"`
	InstalledSize int64  `xml:"InstalledTotalSize,attr"`
	DownloadSize  int64  `xml:"PatchSize,attr"`
}

type patchManifest struct {
	XMLName       xml.Name   `xml:"Manifest"`
	Name          string     `xml:"Name,attr"`
	Build         int        `xml:"Build,attr"`
	FromBuild     int        `xml:"FromBuild,attr"`
	Misc          xmlElement `xml:"Misc"`
	Files         fileGroup  `xml:"Files"`
	PatchFiles    fileGroup  `xml:"PatchFiles"`
	NonCompressed fileGroup  `xml:"NonCompressedFiles"`
	RunCheck      xmlElement `xml:"RunCheck"`
}

type fileGroup struct {
	Files   []manifestFile `xml:"File"`
	Folders []struct {
		Name string `xml:"Name,attr"`
	} `xml:"Folder"`
}

type manifestFile struct {
	Name     string `xml:"Name,attr"`
	Size     int64  `xml:"Size,attr"`
	MD5      string `xml:"MD5,attr"`
	Modified int64  `xml:"LastModified,attr"`
}

type xmlElement struct {
	XMLName  xml.Name
	Attrs    []xml.Attr   `xml:",any,attr"`
	Children []xmlElement `xml:",any"`
}

func readXML(data []byte, value any) error {
	return xml.Unmarshal(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), value)
}

func fetch(ctx *http.Client, url string, max int64) ([]byte, error) {
	resp, err := ctx.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("response too large: %s", url)
	}
	return data, nil
}

func currentBuild(game string) (int, []byte, error) {
	lastPath := filepath.Join(game, "-gup-", "last.xml")
	data, err := os.ReadFile(lastPath)
	if err != nil {
		return 0, nil, err
	}
	var last struct {
		Build int `xml:"Build,attr"`
	}
	if err := readXML(data, &last); err != nil {
		return 0, nil, err
	}
	if last.Build <= 0 {
		return 0, nil, errors.New("invalid installed build in last.xml")
	}
	return last.Build, data, nil
}

type patchHead struct {
	Build int    `xml:"BuildId,attr"`
	Name  string `xml:"Name,attr"`
}

func latestBuild(client *http.Client) (patchHead, error) {
	var h patchHead
	head, err := fetch(client, patchBase+"_head.xml?gid=0.11321", 1<<20)
	if err != nil {
		return h, err
	}
	if err := readXML(head, &h); err != nil {
		return h, err
	}
	if h.Name != "armoredwarfare_hd" || h.Build <= 0 {
		return h, errors.New("unexpected patch head")
	}
	return h, nil
}

func fetchCatalog(client *http.Client, h patchHead) (patchIndex, error) {
	var index patchIndex
	data, err := fetch(client, patchBase+"0.xml?gid=0.11321", 8<<20)
	if err != nil {
		return index, err
	}
	if err := readXML(data, &index); err != nil {
		return index, err
	}
	if index.Name != h.Name {
		return index, errors.New("patch catalog belongs to another game")
	}
	return index, nil
}

func latestDistrib(client *http.Client) (patchInfo, error) {
	h, err := latestBuild(client)
	if err != nil {
		return patchInfo{}, err
	}
	index, err := fetchCatalog(client, h)
	if err != nil {
		return patchInfo{}, err
	}
	for _, d := range index.Distribs {
		if d.Source == 0 && d.Destination == h.Build && d.TorrentURL != "" {
			return d, nil
		}
	}
	return patchInfo{}, fmt.Errorf("the catalog has no full client for build %d", h.Build)
}

func latestPatches(client *http.Client, installed int) ([]patchInfo, int, error) {
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
	predecessor := map[int]patchInfo{}
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
	var path []patchInfo
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

func verifyHexDigest(data []byte, expected string, algorithm string) error {
	var sum []byte
	switch algorithm {
	case "sha1":
		s := sha1.Sum(data)
		sum = s[:]
	case "sha256":
		s := sha256.Sum256(data)
		sum = s[:]
	default:
		return errors.New("unknown digest algorithm")
	}
	if !strings.EqualFold(hex.EncodeToString(sum), expected) {
		return fmt.Errorf("%s mismatch", algorithm)
	}
	return nil
}

func safeGamePath(root, name string) (string, error) {
	if name == "" || strings.ContainsRune(name, ':') {
		return "", fmt.Errorf("unsafe manifest path %q", name)
	}
	clean := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(name, `\`, "/")))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe manifest path %q", name)
	}
	return fitCase(root, clean), nil
}

func setAttr(attrs []xml.Attr, name, value string) []xml.Attr {
	for i := range attrs {
		if attrs[i].Name.Local == name {
			attrs[i].Value = value
			return attrs
		}
	}
	return append(attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
}

func newLastXML(old []byte, patch patchInfo, manifest patchManifest) ([]byte, error) {
	var last xmlElement
	if err := readXML(old, &last); err != nil {
		return nil, err
	}
	if last.XMLName.Local != "Manifest" {
		return nil, errors.New("unexpected last.xml root")
	}
	last.Attrs = setAttr(last.Attrs, "Build", strconv.Itoa(manifest.Build))
	last.Attrs = setAttr(last.Attrs, "VersionUnixTime", strconv.FormatInt(patch.ModifiedUnix, 10))
	last.Attrs = setAttr(last.Attrs, "PatchInstallDate", strconv.FormatInt(time.Now().Unix(), 10))
	last.Attrs = setAttr(last.Attrs, "TimeStamp", strconv.FormatInt(time.Now().Unix(), 10))
	if patch.InstalledSize > 0 {
		last.Attrs = setAttr(last.Attrs, "InstalledSize", strconv.FormatInt(patch.InstalledSize, 10))
	}
	for i := range last.Children {
		switch last.Children[i].XMLName.Local {
		case "Misc":
			last.Children[i] = mergeMisc(last.Children[i], manifest.Misc)
		case "RunCheck":
			last.Children[i] = manifest.RunCheck
		}
	}
	data, err := xml.MarshalIndent(last, "", "\t")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), data...), nil
}
