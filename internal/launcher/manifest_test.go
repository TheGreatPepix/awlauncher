package launcher

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestNewLastXML(t *testing.T) {
	old := []byte(`<?xml version="1.0"?><Manifest Build="441" AutoUpdate="2"><Misc Old="x"/><RunCheck><File Name="old"/></RunCheck><Other Keep="yes"/></Manifest>`)
	m := patchManifest{Build: 442, Misc: xmlElement{XMLName: xml.Name{Local: "Misc"}, Attrs: []xml.Attr{{Name: xml.Name{Local: "New"}, Value: "y"}}}, RunCheck: xmlElement{XMLName: xml.Name{Local: "RunCheck"}}}
	data, err := newLastXML(old, patchInfo{ModifiedUnix: 123, InstalledSize: 456}, m)
	if err != nil {
		t.Fatal(err)
	}
	var root xmlElement
	if err := xml.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, part := range []string{`Build="442"`, `AutoUpdate="2"`, `New="y"`, `Keep="yes"`, `InstalledSize="456"`} {
		if !strings.Contains(s, part) {
			t.Errorf("missing %s in %s", part, s)
		}
	}
	if strings.Contains(s, `Old="x"`) || strings.Contains(s, `Name="old"`) {
		t.Errorf("old metadata retained: %s", s)
	}
}

func TestSafeGamePath(t *testing.T) {
	for _, name := range []string{`..\other`, `C:\other`, `\\server\share`, `a:stream`} {
		if _, err := safeGamePath(`H:\game`, name); err == nil {
			t.Errorf("accepted unsafe path %q", name)
		}
	}
}
