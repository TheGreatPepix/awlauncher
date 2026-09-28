package gamefiles

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const manifestMisc = `<Misc EXEFILENAME64BIT="Bin64\ArmoredWarfare.exe" EXEPARAMS64BIT="-noMultiUI"
	MYCOMCODEPARAM="--sz_token=" MYCOMGAMEACCOUNTPARAM="--sz_pers_id=" EXTDIAG2="bin64\diag.xml"/>`

func TestLaunchConfigReadsUppercaseMisc(t *testing.T) {
	cfg, err := readLaunchConfig([]byte(`<Manifest Build="442">` + manifestMisc + `</Manifest>`))
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Args("7", "C")
	want := []string{"-noMultiUI", "--sz_pers_id=7", "--sz_token=C"}
	if cfg.Exe != `Bin64\ArmoredWarfare.exe` || !reflect.DeepEqual(got, want) {
		t.Fatalf("exe %q args %q", cfg.Exe, got)
	}
}

func TestMergeMiscKeepsGameIDAndCanonicalizes(t *testing.T) {
	var old, manifest xmlElement
	if err := xml.Unmarshal([]byte(`<Misc ExeFileName64bit="old.exe" SezamCodeParam="--gone=" GAMEID="0.11321"/>`), &old); err != nil {
		t.Fatal(err)
	}
	if err := xml.Unmarshal([]byte(manifestMisc), &manifest); err != nil {
		t.Fatal(err)
	}
	merged := mergeMisc(old, manifest)
	names := map[string]string{}
	for _, a := range merged.Attrs {
		names[a.Name.Local] = a.Value
	}
	if names["ExeFileName64bit"] != `Bin64\ArmoredWarfare.exe` || names["GAMEID"] != "0.11321" ||
		names["MyComGameAccountParam"] != "--sz_pers_id=" {
		t.Fatalf("merged = %v", names)
	}
	if _, upper := names["EXEFILENAME64BIT"]; upper || len(merged.Attrs) != 6 {
		t.Fatalf("duplicate or uncanonical names: %v", names)
	}
	if _, stale := names["SezamCodeParam"]; stale {
		t.Fatalf("stale attribute kept: %v", names)
	}
}

func TestInstalledLastXMLOpensAsGame(t *testing.T) {
	var manifest Manifest
	data := `<Manifest Name="armoredwarfare_hd" Build="442">` + manifestMisc +
		`<RunCheck><Item Kind="Hash" Size="1" MD5="00" Name="a"/></RunCheck></Manifest>`
	if err := xml.Unmarshal([]byte(data), &manifest); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	gup := filepath.Join(root, "-gup-")
	if err := os.MkdirAll(gup, 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeInstalledLastXML(gup, patchInfo{Destination: 442, ModifiedUnix: 1, InstalledSize: 2}, manifest); err != nil {
		t.Fatal(err)
	}
	build, last, err := CurrentBuild(root)
	if err != nil || build != 442 {
		t.Fatalf("build %d, %v", build, err)
	}
	cfg, err := readLaunchConfig(last)
	if err != nil || cfg.Exe != `Bin64\ArmoredWarfare.exe` {
		t.Fatalf("launch config %+v, %v", cfg, err)
	}
	for _, want := range []string{`ExeFileName64bit="Bin64\ArmoredWarfare.exe"`, `GAMEID="0.11321"`, `<RunCheck>`} {
		if !strings.Contains(string(last), want) {
			t.Fatalf("last.xml has no %s:\n%s", want, last)
		}
	}
}
