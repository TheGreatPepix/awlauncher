package gamefiles

import (
	"reflect"
	"testing"
)

const sampleLastXML = "\xef\xbb\xbf" + `<?xml version="1.0" encoding="UTF-8"?>
<Manifest AutoUpdate="2" Build="442" Pure="1">
	<Misc MyComCodeParam="--sz_token=" MyComGameAccountParam="--sz_pers_id=" EXTDIAG2="bin64\diag.xml"
	 ExeFileName="" ExeFileName64bit="Bin64\ArmoredWarfare.exe" ExeParams="" ExeParams64bit="-noMultiUI" GAMEID="0.11321"/>
	<RunCheck/>
</Manifest>`

func TestLaunchArgs(t *testing.T) {
	cfg, err := readLaunchConfig([]byte(sampleLastXML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Exe != `Bin64\ArmoredWarfare.exe` {
		t.Fatalf("exe = %q", cfg.Exe)
	}
	got := cfg.Args("123456789", "CODE")
	want := []string{"-noMultiUI", "--sz_pers_id=123456789", "--sz_token=CODE"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %q, want %q", got, want)
	}
}

func TestLaunchConfigPrefersSezam(t *testing.T) {
	xml := `<Manifest Build="1"><Misc ExeFileName64bit="a.exe" SezamCodeParam="--s=" MyComCodeParam="--m="
		SezamGameAccountParam="--xsolla-login-token " MyComGameAccountParam="--p="/></Manifest>`
	cfg, err := readLaunchConfig([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Args("1", "C")
	want := []string{"--xsolla-login-token", "1", "--s=C"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %q, want %q", got, want)
	}
}

func TestLaunchConfigRequiresAuthParams(t *testing.T) {
	if _, err := readLaunchConfig([]byte(`<Manifest Build="1"><Misc ExeFileName64bit="a.exe"/></Manifest>`)); err == nil {
		t.Fatal("expected an error without auth parameters")
	}
}
