package fxid

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/TheGreatPepix/awlauncher/internal/httpx"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/tokens"
)

func SignedIn(resp AuthResponse, email string) (int64, string, error) {
	if resp.Tokens == nil || resp.Tokens.RefreshToken == "" {
		return 0, "", errors.New("FX ID returned no session")
	}
	if resp.Email != "" {
		email = resp.Email
	}
	userID := AccountID(email)
	if err := tokens.Save(userID, resp.Tokens.RefreshToken); err != nil {
		return 0, "", fmt.Errorf("save session: %w", err)
	}
	return userID, email, nil
}

func Session(client *http.Client, userID int64, locale string) (Tokens, error) {
	refresh, err := tokens.Load(userID)
	if errors.Is(err, tokens.ErrNeedLogin) {
		return Tokens{}, err
	}
	if err != nil {
		return Tokens{}, fmt.Errorf("read session: %w", err)
	}
	resp, err := Authenticate(client, AuthRequest{Locale: locale, RefreshToken: refresh})
	if err != nil {
		var status *httpx.StatusError
		if errors.As(err, &status) && status.ClientError() {
			_ = tokens.Clear(userID)
			return Tokens{}, tokens.ErrNeedLogin
		}
		return Tokens{}, fmt.Errorf("refresh session: %w", err)
	}
	if resp.State != StateSuccess || resp.Tokens == nil || resp.Tokens.GameAccessToken == "" {
		_ = tokens.Clear(userID)
		return Tokens{}, tokens.ErrNeedLogin
	}
	if resp.Tokens.RefreshToken != "" && resp.Tokens.RefreshToken != refresh {
		if err := tokens.Save(userID, resp.Tokens.RefreshToken); err != nil {
			return Tokens{}, fmt.Errorf("save session: %w", err)
		}
	}
	return *resp.Tokens, nil
}

func SiteToken(client *http.Client, userID int64, locale string) (string, error) {
	t, err := Session(client, userID, locale)
	if err != nil {
		return "", err
	}
	if t.AccessToken == "" {
		return "", errors.New("FX ID returned no site access token")
	}
	return t.AccessToken, nil
}

func GameToken(client *http.Client, userID int64, locale string) (string, error) {
	t, err := Session(client, userID, locale)
	if err != nil {
		return "", err
	}
	if exp, ok := JWTExpiry(t.GameAccessToken); ok {
		log.Printf("Game token received, valid until %s.", exp.Local().Format("15:04 02.01.2006"))
	}
	return t.GameAccessToken, nil
}

type Branch struct {
	Name     string
	Manifest BranchManifest
	Raw      json.RawMessage
	Err      error
}

func Branches(client *http.Client, siteToken string) ([]Branch, error) {
	names, err := AvailableBranches(client, siteToken)
	if err != nil {
		return nil, fmt.Errorf("branch list: %w", err)
	}
	out := make([]Branch, 0, len(names))
	for _, name := range names {
		b := Branch{Name: name}
		b.Manifest, b.Raw, b.Err = GetBranchManifest(client, siteToken, name)
		out = append(out, b)
	}
	return out, nil
}
