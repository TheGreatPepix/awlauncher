package launcher

import (
	"net/url"
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
	got := cfg.args(gameTicket{GameAccount: "123456789", PersID: "555", Code: "CODE"})
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
	got := cfg.args(gameTicket{GameAccount: "1", Code: "C"})
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

func TestTunnelRoundTrip(t *testing.T) {
	msg := `{"Method":"SetAuthCode","State":"абв"}`
	units := []byte{}
	for _, r := range msg {
		units = append(units, byte(r), byte(r>>8))
	}
	hex := ""
	for _, b := range units {
		hex += string("0123456789abcdef"[b>>4]) + string("0123456789abcdef"[b&15])
	}
	got, err := decodeTunnelParam(hex)
	if err != nil || got != msg {
		t.Fatalf("decode = %q, %v", got, err)
	}
}

func TestAccountsDefaultAndRemove(t *testing.T) {
	var c launcherConfig
	if _, ok := c.defaultAccount(); ok {
		t.Fatal("an empty list must not yield a default account")
	}
	c.upsert(account{UserID: 1, Name: "main"})
	if a, ok := c.defaultAccount(); !ok || a.UserID != 1 {
		t.Fatal("a single account must be the default")
	}
	c.upsert(account{UserID: 2})
	if _, ok := c.defaultAccount(); ok {
		t.Fatal("two accounts without a last choice have no default")
	}
	c.LastUserID = 2
	c.upsert(account{UserID: 1})
	if a, _ := c.find(1); a.Name != "main" {
		t.Fatalf("name lost: %+v", a)
	}
	c.remove(2)
	if c.LastUserID != 0 || len(c.Accounts) != 1 {
		t.Fatalf("after remove: %+v", c)
	}
}

func TestBrowserLoginURLCarriesStateInCallback(t *testing.T) {
	login, _ := url.Parse(browserLoginURL("abc123", false))
	if login.Query().Has("ignore_current_session") {
		t.Fatalf("login = %s, want the browser session kept", login)
	}
	if fresh, _ := url.Parse(browserLoginURL("abc123", true)); fresh.Query().Get("ignore_current_session") != "1" {
		t.Fatalf("fresh login = %s", fresh)
	}
	sdc, _ := url.Parse(login.Query().Get("continue"))
	if sdc.Host != "auth-ac.vkplay.ru" || sdc.Query().Get("state") != "" {
		t.Fatalf("sdc = %s", sdc)
	}
	from, _ := url.Parse(sdc.Query().Get("from"))
	if from.Path != "/gamecenter/authcode/" || from.Query().Get("gc_id") != "0.0" || from.Query().Get("state") != "abc123" {
		t.Fatalf("from = %s", from)
	}
}

func TestConfigStoreChangesOnlyOneField(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	s := newConfigStore(launcherConfig{Accounts: []account{{UserID: 1, Name: "old", Provider: providerFX, Email: "a@b.c"}}})
	held, _ := s.find(1)
	if err := s.updateAccount(1, func(a *account) { a.Name = "new" }); err != nil {
		t.Fatal(err)
	}
	if err := s.updateAccount(held.UserID, func(a *account) { a.Branch = "supertest" }); err != nil {
		t.Fatal(err)
	}
	if a, _ := s.find(1); a.Name != "new" || a.Branch != "supertest" {
		t.Fatalf("account = %+v", a)
	}
	if err := s.update(func(c *launcherConfig) { c.remove(1) }); err != nil {
		t.Fatal(err)
	}
	if err := s.updateAccount(1, func(a *account) { a.Name = "late" }); err != nil {
		t.Fatal(err)
	}
	if len(s.get().Accounts) != 0 {
		t.Fatal("a late change brought the account back")
	}
	saved, err := loadConfig()
	if err != nil || len(saved.Accounts) != 0 {
		t.Fatalf("saved = %+v, %v", saved, err)
	}
}
