package launcher

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fakeJWT(exp int64) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"HS256"}`)) + "." + enc([]byte(fmt.Sprintf(`{"exp":%d}`, exp))) + ".sig"
}

func fakeFXServer(t *testing.T, game string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/authenticate_with_external_id/aw" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request %s %s", r.URL.Path, r.Header.Get("Content-Type"))
		}
		var req fxAuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.GameSystemName != "aw" {
			t.Errorf("bad body: %+v %v", req, err)
		}
		resp := fxAuthResponse{State: 1}
		switch {
		case req.RefreshToken == "refresh-1":
			resp = fxAuthResponse{State: 99, Tokens: &fxTokens{RefreshToken: "refresh-2", GameAccessToken: game}}
		case req.EmailCredentials != nil && req.EmailCredentials.EmailCode == nil:
			resp.State = 3
		case req.EmailCredentials != nil && *req.EmailCredentials.EmailCode == "123456":
			resp = fxAuthResponse{State: 99, Email: "Player@Example.com", Tokens: &fxTokens{RefreshToken: "refresh-1"}}
		case req.EmailCredentials != nil:
			resp.State = 4
		}
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestFXLoginAndGameToken(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	exp := time.Now().Add(time.Hour).Unix()
	srv := fakeFXServer(t, fakeJWT(exp))
	defer srv.Close()
	old := fxBase
	fxBase = srv.URL
	defer func() { fxBase = old }()

	p := prompter{in: bufio.NewReader(strings.NewReader("player@example.com\n654321\n12ab\n123 456\n"))}
	acc, err := loginFX(p, srv.Client(), newConfigStore(launcherConfig{}), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !acc.isFX() || acc.Email != "Player@Example.com" || acc.UserID >= 0 || acc.UserID != fxAccountID("player@example.com") {
		t.Fatalf("account = %+v", acc)
	}
	token, err := fxGameToken(srv.Client(), acc)
	if err != nil || token != fakeJWT(exp) {
		t.Fatalf("token %q, %v", token, err)
	}
	if saved, _ := loadRefreshToken(acc.UserID); saved != "refresh-2" {
		t.Fatalf("saved refresh = %q", saved)
	}
	if _, err := fxGameToken(srv.Client(), acc); err != errNeedLogin {
		t.Fatalf("stale session: %v", err)
	}
	if _, err := loadRefreshToken(acc.UserID); err == nil {
		t.Fatal("stale refresh token kept")
	}
}

func TestFXBranches(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authenticate_with_external_id/aw":
			json.NewEncoder(w).Encode(fxAuthResponse{State: 99, Tokens: &fxTokens{RefreshToken: "r", GameAccessToken: "g", AccessToken: "site"}})
			return
		case "/Account/Login":
			w.Write([]byte("<html>login</html>"))
			return
		}
		if r.Header.Get("Authorization") != "Bearer site" {
			http.Redirect(w, r, "/Account/Login", http.StatusFound)
			return
		}
		switch r.URL.Path {
		case "/api/v1/ftl_integration/aw/launcher/available_branches":
			w.Write([]byte(`{"Branches":[{"Name":"default"},{"Name":"supertest"}]}`))
		case "/api/v1/ftl_integration/aw/launcher/manifest/default":
			w.Write([]byte(`{"Manifest":{"release":{"buildNumber":1001576,"buildVersion":"0.566.1","createdAt":"2026-09-25T19:01:32Z"},"artifacts":[{"fullSize":73336340642}]},"AvailableBranches":["default"]}`))
		default:
			http.Redirect(w, r, "/Account/Login", http.StatusFound)
		}
	}))
	defer srv.Close()
	old := fxBase
	fxBase = srv.URL
	defer func() { fxBase = old }()
	acc := account{UserID: fxAccountID("a@b"), Provider: providerFX, Email: "a@b"}
	if err := saveRefreshToken(acc.UserID, "r"); err != nil {
		t.Fatal(err)
	}
	branches, err := fxBranches(srv.Client(), acc)
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 || branches[0].Version != "0.566.1" || branches[0].Build != 1001576 || branches[0].FullSize != 73336340642 {
		t.Fatalf("branches = %+v", branches)
	}
	if !errors.Is(branches[1].Err, errFXNoAccess) {
		t.Fatalf("supertest manifest error = %v", branches[1].Err)
	}
}

func TestFXActivateKey(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/authenticate_with_external_id/aw" {
			json.NewEncoder(w).Encode(fxAuthResponse{State: 99, Tokens: &fxTokens{RefreshToken: "r", GameAccessToken: "g", AccessToken: "site"}})
			return
		}
		if r.URL.Path != "/api/v1/ftl_integration/aw/launcher/activate_key" || r.Method != http.MethodPost ||
			r.Header.Get("Authorization") != "Bearer site" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body struct{ Key string }
		json.NewDecoder(r.Body).Decode(&body)
		if body.Key == "GOOD-KEY" {
			w.Write([]byte(`{"ActivatedBranch":"supertest"}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"title":"Bad Request","status":400,"errors":[{"name":"GeneralErrors","reason":"Key not found"}]}`))
	}))
	defer srv.Close()
	old := fxBase
	fxBase = srv.URL
	defer func() { fxBase = old }()
	acc := account{UserID: fxAccountID("a@b"), Provider: providerFX, Email: "a@b"}
	if err := saveRefreshToken(acc.UserID, "r"); err != nil {
		t.Fatal(err)
	}
	if branch, err := fxActivateKey(srv.Client(), acc, "GOOD-KEY"); err != nil || branch != "supertest" {
		t.Fatalf("good key: %q, %v", branch, err)
	}
	if _, err := fxActivateKey(srv.Client(), acc, "BAD"); err == nil || !strings.Contains(err.Error(), "Key not found") {
		t.Fatalf("bad key error = %v", err)
	}
}

func TestFXLaunchArgs(t *testing.T) {
	args := fxLaunchArgs(fxLaunchDefault, account{Email: "a@b"}, "JWT")
	lang := map[string]string{"ru": "russian", "en": "english"}[uiLanguage()]
	if want := []string{"-pref_language", lang, "--fxid-login-token=JWT"}; !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %q, want %q", args, want)
	}
}

func TestJWTExpiry(t *testing.T) {
	if exp, ok := jwtExpiry(fakeJWT(1790000000)); !ok || exp.Unix() != 1790000000 {
		t.Fatalf("exp = %v %v", exp, ok)
	}
	if _, ok := jwtExpiry("not-a-jwt"); ok {
		t.Fatal("accepted a non-JWT")
	}
}

func TestFirstWord(t *testing.T) {
	for in, want := range map[string]string{"1 play": "1", "  Q ": "q", "": "", "-2": "-2"} {
		if got := firstWord(in); got != want {
			t.Errorf("firstWord(%q) = %q, want %q", in, got, want)
		}
	}
}
