package fxid

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticateSteamOpenIDSendsTheFullCallback(t *testing.T) {
	callback := "http://127.0.0.1:12345/?state=nonce&openid.mode=id_res&openid.sig=proof"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/authenticate_to_fxid_adapter_for_external_id" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var request SteamOpenIDAuthRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.AuthenticationType != 5 || request.Game != "aw" || request.Locale != "en" || request.SteamOpenIdLogin.OriginURL != callback {
			t.Errorf("unexpected auth request: %+v", request)
		}
		_ = json.NewEncoder(w).Encode(AuthResponse{State: StateSuccess, Tokens: &Tokens{GameAccessToken: "game"}})
	}))
	defer server.Close()
	old := Base
	Base = server.URL
	defer func() { Base = old }()

	response, err := AuthenticateSteamOpenID(server.Client(), callback, "en")
	if err != nil || response.State != StateSuccess || response.Tokens == nil || response.Tokens.GameAccessToken != "game" {
		t.Fatalf("response = %+v, error = %v", response, err)
	}
}
