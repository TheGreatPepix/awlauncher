package vkplay

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"net/url"

	"github.com/TheGreatPepix/awlauncher/internal/httpx"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/vkplay/bridge"
)

const (
	GameProjectID = "11321"
	OAuthClientID = "gc.my.com"
	gameAgent     = "Downloader/19160"
)

type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int64  `json:"user_id"`
}

type GameTicket struct {
	XMLName     xml.Name `xml:"Login"`
	GameAccount string   `xml:"GameAccount,attr"`
	PersID      string   `xml:"PersId,attr"`
	Code        string   `xml:"Code,attr"`
}

func ExchangeBrowserCode(client *http.Client, code string) (Tokens, error) {
	var tokens Tokens
	if !bridge.ValidCode(code) {
		return tokens, errors.New("invalid sign-in code")
	}
	body, _ := json.Marshal(map[string]string{"client_id": OAuthClientID, "code": code})
	data, err := httpx.Post(client, "https://o2-ext-ac.vkplay.ru/api/v3/pub/oauth2/token", "application/json", body, httpx.BrowserAgent)
	if err != nil {
		return tokens, err
	}
	if err := json.Unmarshal(data, &tokens); err != nil {
		return tokens, errors.New("unexpected OAuth response")
	}
	if !bridge.ValidCode(tokens.RefreshToken) || tokens.UserID <= 0 {
		return Tokens{}, errors.New("OAuth returned no refresh_token")
	}
	return tokens, nil
}

func RefreshSession(client *http.Client, refreshToken string) (string, string, error) {
	if !bridge.ValidCode(refreshToken) {
		return "", "", errors.New("invalid refresh_token")
	}
	form := url.Values{"client_id": {OAuthClientID}, "grant_type": {"refresh_token"}, "refresh_token": {refreshToken}}
	data, err := httpx.Post(client, "https://o2-ac.vkplay.ru/token", "application/x-www-form-urlencoded", []byte(form.Encode()), httpx.BrowserAgent)
	if err != nil {
		return "", "", err
	}
	var response struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if json.Unmarshal(data, &response) != nil || !bridge.ValidCode(response.AccessToken) {
		return "", "", errors.New("unexpected O2 response")
	}
	if !bridge.ValidCode(response.RefreshToken) {
		response.RefreshToken = ""
	}
	return response.AccessToken, response.RefreshToken, nil
}

func RequestGameTicket(client *http.Client, sessionKey string) (GameTicket, error) {
	var ticket GameTicket
	if !bridge.ValidCode(sessionKey) {
		return ticket, errors.New("invalid SessionKey")
	}
	body, err := xml.Marshal(struct {
		XMLName    xml.Name `xml:"Login"`
		SessionKey string   `xml:"SessionKey,attr"`
		ProjectID  string   `xml:"ProjectId,attr"`
	}{SessionKey: sessionKey, ProjectID: GameProjectID})
	if err != nil {
		return ticket, err
	}
	data, err := httpx.Post(client, "https://authdl.vkplay.ru/gem.php?hint=Login", "text/xml", body, gameAgent)
	if err != nil {
		return ticket, err
	}
	if xml.Unmarshal(data, &ticket) != nil || ticket.XMLName.Local != "Login" ||
		!bridge.ValidCode(ticket.Code) || ticket.GameAccount == "" {
		return GameTicket{}, errors.New("unexpected game token response")
	}
	return ticket, nil
}
