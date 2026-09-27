package launcher

import (
	"crypto/sha1"
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
	got := cfg.args(gameTicket{GameAccount: "7", Code: "C"})
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
	var manifest patchManifest
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
	build, last, err := currentBuild(root)
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

func TestStaleCacheEntriesKeepsOneVersion(t *testing.T) {
	names := []string{
		"payload-440-441", "stage-440-441", "backup-440-441-20260901-100000",
		"payload-441-442", "stage-441-442", "backup-441-442-20260926-114944",
		"payload-442-443", "stage-442-443",
		"notes", "backup-x-y",
	}
	got := staleCacheEntries(names, 442)
	want := []string{"payload-440-441", "stage-440-441", "backup-440-441-20260901-100000", "stage-441-442"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stale = %v, want %v", got, want)
	}
}

func TestBadFilesFindsCorruptFile(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{"a.bin": "0123456789", "b.bin": "abcdefghij", "c.bin": "ABCDEFGHIJ"}
	meta := torrentMeta{pieceSize: 8}
	var stream string
	for _, name := range []string{"a.bin", "b.bin", "c.bin"} {
		meta.files = append(meta.files, torrentFile{name: name, size: int64(len(files[name]))})
		stream += files[name]
		if err := os.WriteFile(filepath.Join(root, name), []byte(files[name]), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < len(stream); i += 8 {
		end := min(i+8, len(stream))
		sum := sha1.Sum([]byte(stream[i:end]))
		meta.hashes = append(meta.hashes, sum[:]...)
	}
	if bad, err := meta.badFiles(root); err != nil || len(bad) != 0 {
		t.Fatalf("clean: %v, %v", bad, err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.bin"), []byte("abXdefghij"), 0644); err != nil {
		t.Fatal(err)
	}
	bad, err := meta.badFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.bin", "b.bin"}; !reflect.DeepEqual(bad, want) {
		t.Fatalf("bad = %v, want %v", bad, want)
	}
	if meta.verifyPieces(root) == nil {
		t.Fatal("verifyPieces accepted a corrupt file")
	}
}
