package steam

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/platform"
)

type Assertion struct {
	UserID    string
	OriginURL string
}

var ErrCancelled = errors.New("Steam browser sign-in was not completed")

func SignIn() (Assertion, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return Assertion{}, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Assertion{}, err
	}
	defer listener.Close()
	base := "http://" + listener.Addr().String() + "/"
	state := hex.EncodeToString(nonce[:])
	callback := base + "?state=" + state
	returned := make(chan Assertion, 1)
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path != "/" || r.URL.Query().Get("state") != state || r.URL.Query().Get("openid.mode") != "id_res" {
			http.Error(w, "Invalid Steam response", http.StatusBadRequest)
			return
		}
		id, err := steamID(r.URL.Query().Get("openid.claimed_id"))
		if err != nil {
			http.Error(w, "Invalid Steam account", http.StatusBadRequest)
			return
		}
		assertion := Assertion{UserID: id, OriginURL: strings.TrimSuffix(base, "/") + r.URL.RequestURI()}
		select {
		case returned <- assertion:
			_, _ = fmt.Fprint(w, "Steam sign-in received. You can return to AWLauncher.")
		default:
			_, _ = fmt.Fprint(w, "Steam sign-in already received.")
		}
	})
	go func() { _ = server.Serve(listener) }()
	defer server.Shutdown(context.Background())

	q := url.Values{
		"openid.ns":         {"http://specs.openid.net/auth/2.0"},
		"openid.mode":       {"checkid_setup"},
		"openid.return_to":  {callback},
		"openid.realm":      {base},
		"openid.identity":   {"http://specs.openid.net/auth/2.0/identifier_select"},
		"openid.claimed_id": {"http://specs.openid.net/auth/2.0/identifier_select"},
	}
	if err := platform.OpenBrowser("https://steamcommunity.com/openid/login?" + q.Encode()); err != nil {
		return Assertion{}, err
	}
	select {
	case assertion := <-returned:
		return assertion, nil
	case <-time.After(5 * time.Minute):
		return Assertion{}, ErrCancelled
	}
}

func steamID(claimed string) (string, error) {
	u, err := url.Parse(claimed)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host != "steamcommunity.com" ||
		u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, "/openid/id/") {
		return "", errors.New("invalid Steam OpenID identity")
	}
	id := strings.TrimPrefix(u.Path, "/openid/id/")
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil || n == 0 || strconv.FormatUint(n, 10) != id {
		return "", errors.New("invalid SteamID")
	}
	return id, nil
}
