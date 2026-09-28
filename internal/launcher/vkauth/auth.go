package vkauth

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/vkbridge"
)

const (
	GameProjectID = "11321"
	OAuthClientID = "gc.my.com"
	gameAgent     = "Downloader/19160"
	BrowserAgent  = "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.5414.120 Downloader/19160 MyComGameCenter/1916 Safari/537.36"
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

type HTTPStatusError struct {
	Host string
	Code int
}

func (e *HTTPStatusError) Error() string { return fmt.Sprintf("%s: HTTP %d", e.Host, e.Code) }

func Post(client *http.Client, endpoint, contentType string, body []byte, agent string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", agent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPStatusError{Host: req.URL.Host, Code: resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, errors.New("auth response too large")
	}
	return data, nil
}

func ExchangeBrowserCode(client *http.Client, code string) (Tokens, error) {
	var tokens Tokens
	if !vkbridge.ValidCode(code) {
		return tokens, errors.New("invalid sign-in code")
	}
	body, _ := json.Marshal(map[string]string{"client_id": OAuthClientID, "code": code})
	data, err := Post(client, "https://o2-ext-ac.vkplay.ru/api/v3/pub/oauth2/token", "application/json", body, BrowserAgent)
	if err != nil {
		return tokens, err
	}
	if err := json.Unmarshal(data, &tokens); err != nil {
		return tokens, errors.New("unexpected OAuth response")
	}
	if !vkbridge.ValidCode(tokens.RefreshToken) || tokens.UserID <= 0 {
		return Tokens{}, errors.New("OAuth returned no refresh_token")
	}
	return tokens, nil
}

func RefreshSession(client *http.Client, refreshToken string) (string, string, error) {
	if !vkbridge.ValidCode(refreshToken) {
		return "", "", errors.New("invalid refresh_token")
	}
	form := url.Values{"client_id": {OAuthClientID}, "grant_type": {"refresh_token"}, "refresh_token": {refreshToken}}
	data, err := Post(client, "https://o2-ac.vkplay.ru/token", "application/x-www-form-urlencoded", []byte(form.Encode()), BrowserAgent)
	if err != nil {
		return "", "", err
	}
	var response struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if json.Unmarshal(data, &response) != nil || !vkbridge.ValidCode(response.AccessToken) {
		return "", "", errors.New("unexpected O2 response")
	}
	if !vkbridge.ValidCode(response.RefreshToken) {
		response.RefreshToken = ""
	}
	return response.AccessToken, response.RefreshToken, nil
}

func RequestGameTicket(client *http.Client, sessionKey string) (GameTicket, error) {
	var ticket GameTicket
	if !vkbridge.ValidCode(sessionKey) {
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
	data, err := Post(client, "https://authdl.vkplay.ru/gem.php?hint=Login", "text/xml", body, gameAgent)
	if err != nil {
		return ticket, err
	}
	if xml.Unmarshal(data, &ticket) != nil || ticket.XMLName.Local != "Login" ||
		!vkbridge.ValidCode(ticket.Code) || ticket.GameAccount == "" {
		return GameTicket{}, errors.New("unexpected game token response")
	}
	return ticket, nil
}
