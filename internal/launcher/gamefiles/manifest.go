package gamefiles

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/vkplay"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
)

type patchInfo = vkplay.PatchInfo

type Manifest struct {
	XMLName       xml.Name   `xml:"Manifest"`
	Name          string     `xml:"Name,attr"`
	Build         int        `xml:"Build,attr"`
	FromBuild     int        `xml:"FromBuild,attr"`
	Misc          xmlElement `xml:"Misc"`
	Files         FileGroup  `xml:"Files"`
	PatchFiles    FileGroup  `xml:"PatchFiles"`
	NonCompressed FileGroup  `xml:"NonCompressedFiles"`
	RunCheck      xmlElement `xml:"RunCheck"`
}

type FileGroup struct {
	Files   []ManifestFile `xml:"File"`
	Folders []struct {
		Name string `xml:"Name,attr"`
	} `xml:"Folder"`
}

type ManifestFile struct {
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

func CurrentBuild(game string) (int, []byte, error) {
	lastPath := filepath.Join(game, "-gup-", "last.xml")
	data, err := os.ReadFile(lastPath)
	if err != nil {
		return 0, nil, err
	}
	var last struct {
		Build int `xml:"Build,attr"`
	}
	if err := vkplay.ReadXML(data, &last); err != nil {
		return 0, nil, err
	}
	if last.Build <= 0 {
		return 0, nil, errors.New("invalid installed build in last.xml")
	}
	return last.Build, data, nil
}

func SafePath(root, name string) (string, error) {
	if name == "" || strings.ContainsRune(name, ':') {
		return "", fmt.Errorf("unsafe manifest path %q", name)
	}
	clean := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(name, `\`, "/")))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe manifest path %q", name)
	}
	return platform.FitCase(root, clean), nil
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

func newLastXML(old []byte, patch patchInfo, manifest Manifest) ([]byte, error) {
	var last xmlElement
	if err := vkplay.ReadXML(old, &last); err != nil {
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
