package launcher

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/fxid"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/tokens"
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
		var req fxid.AuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.GameSystemName != "aw" {
			t.Errorf("bad body: %+v %v", req, err)
		}
		resp := fxid.AuthResponse{State: 1}
		switch {
		case req.RefreshToken == "refresh-1":
			resp = fxid.AuthResponse{State: 99, Tokens: &fxid.Tokens{RefreshToken: "refresh-2", GameAccessToken: game}}
		case req.EmailCredentials != nil && req.EmailCredentials.EmailCode == nil:
			resp.State = 3
		case req.EmailCredentials != nil && *req.EmailCredentials.EmailCode == "123456":
			resp = fxid.AuthResponse{State: 99, Email: "Player@Example.com", Tokens: &fxid.Tokens{RefreshToken: "refresh-1"}}
		case req.EmailCredentials != nil:
			resp.State = 4
		}
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestFXLoginAndGameToken(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dataDir)
	t.Setenv("XDG_DATA_HOME", dataDir)
	exp := time.Now().Add(time.Hour).Unix()
	srv := fakeFXServer(t, fakeJWT(exp))
	defer srv.Close()
	old := fxid.Base
	fxid.Base = srv.URL
	defer func() { fxid.Base = old }()

	ui := &fakeUI{answers: []string{"player@example.com", "654321", "12ab", "123 456"}}
	s := &Session{ui: ui, cfg: config.NewStore(config.Config{}), client: srv.Client(), found: &foundGame{}}
	acc, err := s.loginFX("", "")
	if err != nil {
		t.Fatal(err)
	}
	if !acc.IsFX() || acc.Email != "Player@Example.com" || acc.UserID >= 0 || acc.UserID != fxid.AccountID("player@example.com") {
		t.Fatalf("account = %+v", acc)
	}
	var kinds []PromptKind
	for _, p := range ui.asked {
		kinds = append(kinds, p.Kind)
	}
	if fmt.Sprint(kinds) != fmt.Sprint([]PromptKind{PromptEmail, PromptCode, PromptCode, PromptCode}) {
		t.Fatalf("prompts = %v", kinds)
	}
	token, err := fxid.GameToken(srv.Client(), acc.UserID, "en")
	if err != nil || token != fakeJWT(exp) {
		t.Fatalf("token %q, %v", token, err)
	}
	if saved, _ := tokens.Load(acc.UserID); saved != "refresh-2" {
		t.Fatalf("saved refresh = %q", saved)
	}
	if _, err := fxid.GameToken(srv.Client(), acc.UserID, "en"); err != ErrNeedLogin {
		t.Fatalf("stale session: %v", err)
	}
	if _, err := tokens.Load(acc.UserID); err == nil {
		t.Fatal("stale refresh token kept")
	}
}

func TestFXBranches(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dataDir)
	t.Setenv("XDG_DATA_HOME", dataDir)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authenticate_with_external_id/aw":
			json.NewEncoder(w).Encode(fxid.AuthResponse{State: 99, Tokens: &fxid.Tokens{RefreshToken: "r", GameAccessToken: "g", AccessToken: "site"}})
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
	old := fxid.Base
	fxid.Base = srv.URL
	defer func() { fxid.Base = old }()
	acc := config.Account{UserID: fxid.AccountID("a@b"), Provider: config.ProviderFX, Email: "a@b"}
	if err := tokens.Save(acc.UserID, "r"); err != nil {
		t.Fatal(err)
	}
	branches, err := fxBranches(srv.Client(), acc)
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 || branches[0].Manifest.Version() != "0.566.1" || branches[0].Manifest.Manifest.Release.BuildNumber != 1001576 || branches[0].Manifest.FullSize() != 73336340642 {
		t.Fatalf("branches = %+v", branches)
	}
	if !errors.Is(branches[1].Err, fxid.ErrNoAccess) {
		t.Fatalf("supertest manifest error = %v", branches[1].Err)
	}
}

func TestFXRemoveAccountSignsOut(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dataDir)
	t.Setenv("XDG_DATA_HOME", dataDir)
	loggedOut := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/authenticate_with_external_id/aw":
			json.NewEncoder(w).Encode(fxid.AuthResponse{State: 99, Tokens: &fxid.Tokens{RefreshToken: "r", GameAccessToken: "g", AccessToken: "site"}})
		case "/api/v1/auth/logout":
			if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer site" {
				t.Errorf("logout %s with %q", r.Method, r.Header.Get("Authorization"))
			}
			loggedOut = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	old := fxid.Base
	fxid.Base = srv.URL
	defer func() { fxid.Base = old }()
	acc := config.Account{UserID: fxid.AccountID("a@b"), Provider: config.ProviderFX, Email: "a@b"}
	if err := tokens.Save(acc.UserID, "r"); err != nil {
		t.Fatal(err)
	}
	s := &Session{ui: &fakeUI{}, cfg: config.NewStore(config.Config{Accounts: []config.Account{acc}}), client: srv.Client(), found: &foundGame{}}
	if err := s.RemoveAccount(acc); err != nil {
		t.Fatal(err)
	}
	if !loggedOut {
		t.Fatal("logout was not called")
	}
	if _, err := tokens.Load(acc.UserID); !errors.Is(err, tokens.ErrNeedLogin) {
		t.Fatalf("token kept: %v", err)
	}
	if _, ok := s.cfg.Find(acc.UserID); ok {
		t.Fatal("account kept in config")
	}
}

func TestFXActivateKey(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dataDir)
	t.Setenv("XDG_DATA_HOME", dataDir)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/authenticate_with_external_id/aw" {
			json.NewEncoder(w).Encode(fxid.AuthResponse{State: 99, Tokens: &fxid.Tokens{RefreshToken: "r", GameAccessToken: "g", AccessToken: "site"}})
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
	old := fxid.Base
	fxid.Base = srv.URL
	defer func() { fxid.Base = old }()
	acc := config.Account{UserID: fxid.AccountID("a@b"), Provider: config.ProviderFX, Email: "a@b"}
	if err := tokens.Save(acc.UserID, "r"); err != nil {
		t.Fatal(err)
	}
	if branch, err := fxActivateKey(srv.Client(), acc, "GOOD-KEY"); err != nil || branch != "supertest" {
		t.Fatalf("good key: %q, %v", branch, err)
	}
	if _, err := fxActivateKey(srv.Client(), acc, "BAD"); err == nil || !strings.Contains(err.Error(), "Key not found") {
		t.Fatalf("bad key error = %v", err)
	}
}
