package launcher

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/fxid"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/steam"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/tokens"
)

func steamJWT(exp int64) string {
	payload := fmt.Sprintf(`{"exp":%d,"soc_ids":{"steam":"12345678901234567"}}`, exp)
	return "e30." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"
}

func fakeSteamBrowser(t *testing.T, id string) *int {
	t.Helper()
	old := steamSignIn
	calls := new(int)
	steamSignIn = func() (steam.Assertion, error) {
		*calls++
		return steam.Assertion{UserID: id, OriginURL: "http://127.0.0.1/?openid.sig=proof"}, nil
	}
	t.Cleanup(func() { steamSignIn = old })
	return calls
}

func TestSteamBrowserSessionIsCachedAndExpiredSessionNeedsLogin(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	calls := fakeSteamBrowser(t, "12345678901234567")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/authenticate_to_fxid_adapter_for_external_id" {
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
		var request fxid.SteamOpenIDAuthRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.AuthenticationType != 5 || request.Game != "aw" || request.SteamOpenIdLogin.OriginURL == "" {
			t.Errorf("unexpected OpenID request: %+v", request)
		}
		jwt := steamJWT(time.Now().Add(time.Hour).Unix())
		_ = json.NewEncoder(w).Encode(fxid.AuthResponse{State: fxid.StateSuccess, Tokens: &fxid.Tokens{GameAccessToken: jwt, AccessToken: jwt, RefreshToken: "unusable"}})
	}))
	defer server.Close()
	oldBase := fxid.Base
	fxid.Base = server.URL
	defer func() { fxid.Base = oldBase }()
	s := &Session{ui: &fakeUI{}, cfg: config.NewStore(config.Config{}), client: server.Client(), found: &foundGame{}}
	acc, err := s.loginSteam(config.Account{})
	if err != nil || !acc.IsSteam() || *calls != 1 {
		t.Fatalf("account = %+v, calls = %d, error = %v", acc, *calls, err)
	}
	if _, err := cachedSteamTokens(acc.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.gameToken(acc); err != nil || *calls != 1 {
		t.Fatalf("cached game token: calls = %d, error = %v", *calls, err)
	}
	if _, err := s.siteToken(acc); err != nil || *calls != 1 {
		t.Fatalf("cached site token: calls = %d, error = %v", *calls, err)
	}
	if err := saveSteamTokens(acc.UserID, fxid.Tokens{GameAccessToken: steamJWT(time.Now().Add(time.Minute).Unix()), AccessToken: steamJWT(time.Now().Add(time.Hour).Unix())}); err != nil {
		t.Fatal(err)
	}
	if _, err := cachedSteamTokens(acc.UserID); !errors.Is(err, ErrNeedLogin) {
		t.Fatalf("expired session: %v", err)
	}
	if err := saveSteamTokens(23456789012345678, fxid.Tokens{GameAccessToken: steamJWT(time.Now().Add(time.Hour).Unix()), AccessToken: steamJWT(time.Now().Add(time.Hour).Unix())}); err != nil {
		t.Fatal(err)
	}
	if _, err := cachedSteamTokens(23456789012345678); !errors.Is(err, ErrNeedLogin) {
		t.Fatalf("session for another account: %v", err)
	}
	if err := s.RemoveAccount(acc); err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.Load(acc.UserID); !errors.Is(err, ErrNeedLogin) {
		t.Fatalf("removed session: %v", err)
	}
}

func TestSteamBrowserRejectsDifferentAccount(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	fakeSteamBrowser(t, "12345678901234567")
	s := &Session{ui: &fakeUI{}, cfg: config.NewStore(config.Config{}), client: http.DefaultClient, found: &foundGame{}}
	if _, err := s.loginSteam(config.Account{UserID: 23456789012345678, Provider: config.ProviderSteam}); err == nil {
		t.Fatal("accepted a different Steam account")
	}
}

func TestSteamGameEnvUsesAccountID(t *testing.T) {
	acc := config.Account{UserID: 12345678901234567, Provider: config.ProviderSteam}
	if got, want := fxGameEnv(acc, steamJWT(time.Now().Add(time.Hour).Unix())), []string{"GC_PERS_ID=12345678901234567"}; !slices.Equal(got, want) {
		t.Fatalf("Steam game environment = %q, want %q", got, want)
	}
}
