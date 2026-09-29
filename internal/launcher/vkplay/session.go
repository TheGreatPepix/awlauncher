package vkplay

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/TheGreatPepix/awlauncher/internal/httpx"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/tokens"
)

func SignIn(client *http.Client, code string) (int64, error) {
	t, err := ExchangeBrowserCode(client, code)
	if err != nil {
		return 0, err
	}
	if err := tokens.Save(t.UserID, t.RefreshToken); err != nil {
		return 0, fmt.Errorf("save session: %w", err)
	}
	return t.UserID, nil
}

func SessionKey(client *http.Client, userID int64) (string, error) {
	token, err := tokens.Load(userID)
	if errors.Is(err, tokens.ErrNeedLogin) {
		return "", err
	}
	if err != nil {
		return "", fmt.Errorf("read session: %w", err)
	}
	key, rotated, err := RefreshSession(client, token)
	if err != nil {
		var status *httpx.StatusError
		if errors.As(err, &status) && status.ClientError() {
			_ = tokens.Clear(userID)
			return "", tokens.ErrNeedLogin
		}
		return "", fmt.Errorf("refresh session: %w", err)
	}
	if rotated != "" && rotated != token {
		if err := tokens.Save(userID, rotated); err != nil {
			return "", fmt.Errorf("save session: %w", err)
		}
	}
	return key, nil
}

func Ticket(client *http.Client, userID int64) (GameTicket, error) {
	key, err := SessionKey(client, userID)
	if err != nil {
		return GameTicket{}, err
	}
	return RequestGameTicket(client, key)
}

func SignOut(client *http.Client, userID int64) error {
	if token, err := tokens.Load(userID); err == nil {
		body, _ := json.Marshal(map[string]string{"client_id": OAuthClientID, "refresh_token": token})
		if _, err := httpx.Post(client, "https://o2-ext-ac.vkplay.ru/api/v3/pub/oauth2/drop", "application/json", body, httpx.BrowserAgent); err != nil {
			log.Print("Warning: the server did not confirm session revocation: ", err)
		}
	}
	return tokens.Clear(userID)
}
