package fxid

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/httpx"
)

var Base = "https://id.wishlistgames.net"

const (
	Game          = "aw"
	Platform      = "11"
	LaunchDefault = "{original_command_line_param} {launchParams} -pref_language {languageName} --{token_name}={token_value}"
)

const (
	StateNeedEmailCode = 3
	StateWrongCode     = 4
	StateSuccess       = 99
)

var stateNames = map[int]string{
	1: "no credentials", 2: "wrong e-mail for this account", 5: "unknown server error",
	6: "validation error", 7: "invite error", 9: "account has no e-mail", 10: "account has no e-mail",
	11: "too many requests, try again later", 12: "e-mail is already taken", 13: "account is banned",
}

var codePattern = regexp.MustCompile(`^[0-9]{6}$`)

func ValidCode(code string) bool { return codePattern.MatchString(code) }

var branchNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func ValidBranchName(name string) bool { return branchNamePattern.MatchString(name) }

type EmailCredentials struct {
	Email     string  `json:"Email"`
	EmailCode *string `json:"EmailCode"`
}

type AuthRequest struct {
	GameSystemName    string `json:"GameSystemName"`
	Locale            string `json:"Locale"`
	SessionID         string `json:"SessionId"`
	RefreshToken      string `json:"RefreshToken,omitempty"`
	AdditionalOptions struct {
		AllowOnlyNewEmail bool `json:"AllowOnlyNewEmail"`
	} `json:"AdditionalOptions"`
	EmailCredentials *EmailCredentials `json:"EmailCredentials,omitempty"`
}

type Tokens struct {
	RefreshToken    string `json:"RefreshToken"`
	GameAccessToken string `json:"GameAccessToken"`
	AccessToken     string `json:"AccessToken"`
}

type AuthResponse struct {
	State  int     `json:"State"`
	Tokens *Tokens `json:"Tokens"`
	Hint   string  `json:"Hint"`
	Email  string  `json:"Email"`
}

func (r AuthResponse) Failure() error {
	name := stateNames[r.State]
	if name == "" {
		name = fmt.Sprintf("state %d", r.State)
	}
	if r.Hint != "" {
		return fmt.Errorf("FX ID: %s (%s)", name, r.Hint)
	}
	return fmt.Errorf("FX ID: %s", name)
}

func Authenticate(client *http.Client, req AuthRequest) (AuthResponse, error) {
	var resp AuthResponse
	req.GameSystemName = Game
	body, err := json.Marshal(req)
	if err != nil {
		return resp, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, Base+"/api/v1/auth/authenticate_with_external_id/"+Game, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", httpx.BrowserAgent)
	r, err := client.Do(httpReq)
	if err != nil {
		return resp, err
	}
	defer r.Body.Close()
	data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return resp, err
	}
	if r.StatusCode != http.StatusOK {
		return resp, &httpx.StatusError{Host: httpReq.URL.Host, Code: r.StatusCode}
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.New("unexpected FX ID response")
	}
	return resp, nil
}

func AccountID(email string) int64 {
	h := fnv.New64a()
	h.Write([]byte(strings.ToLower(strings.TrimSpace(email))))
	return -int64(h.Sum64() >> 1)
}

var ErrNoAccess = errors.New("no access")

func call(client *http.Client, method, path, accessToken string, body, out any) error {
	noRedirect := *client
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, Base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", httpx.BrowserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := noRedirect.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden,
		resp.StatusCode >= 300 && resp.StatusCode < 400:
		return fmt.Errorf("%w (HTTP %d)", ErrNoAccess, resp.StatusCode)
	default:
		if msg := problemText(data); msg != "" {
			return fmt.Errorf("%s (HTTP %d)", msg, resp.StatusCode)
		}
		return &httpx.StatusError{Host: req.URL.Host, Code: resp.StatusCode}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

func get(client *http.Client, path, accessToken string, out any) error {
	return call(client, http.MethodGet, path, accessToken, nil, out)
}

func problemText(data []byte) string {
	var p struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
		Errors []struct {
			Reason string `json:"reason"`
		} `json:"errors"`
	}
	if json.Unmarshal(data, &p) != nil {
		return ""
	}
	var reasons []string
	for _, e := range p.Errors {
		if e.Reason != "" {
			reasons = append(reasons, e.Reason)
		}
	}
	switch {
	case len(reasons) > 0:
		return strings.Join(reasons, "; ")
	case p.Detail != "":
		return p.Detail
	}
	return p.Title
}

func ActivateKey(client *http.Client, accessToken, key string) (string, error) {
	var resp struct {
		ActivatedBranch string `json:"ActivatedBranch"`
	}
	body := struct {
		Key string `json:"Key"`
	}{Key: key}
	if err := call(client, http.MethodPost, "/api/v1/ftl_integration/"+Game+"/launcher/activate_key", accessToken, body, &resp); err != nil {
		return "", err
	}
	if resp.ActivatedBranch == "" {
		return "", errors.New("the key was not accepted")
	}
	return resp.ActivatedBranch, nil
}

func AvailableBranches(client *http.Client, accessToken string) ([]string, error) {
	var list struct {
		Branches []struct {
			Name string `json:"Name"`
		} `json:"Branches"`
	}
	if err := get(client, "/api/v1/ftl_integration/"+Game+"/launcher/available_branches", accessToken, &list); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Branches))
	for _, b := range list.Branches {
		names = append(names, b.Name)
	}
	return names, nil
}

type BranchManifest struct {
	Manifest struct {
		LauncherConfiguration struct {
			LaunchFile   string `json:"launchFile"`
			HTTPDownload struct {
				DownloadBaseURI string `json:"downloadBaseUri"`
			} `json:"httpDownload"`
			ManifestURL    string `json:"manifestUrl"`
			ManifestSHA256 string `json:"manifestSha256"`
		} `json:"launcherConfiguration"`
		Release struct {
			BuildNumber  int64     `json:"buildNumber"`
			BuildVersion string    `json:"buildVersion"`
			CreatedAt    time.Time `json:"createdAt"`
		} `json:"release"`
		Artifacts []struct {
			FullSize int64 `json:"fullSize"`
		} `json:"artifacts"`
	} `json:"Manifest"`
}

func (m BranchManifest) Version() string { return m.Manifest.Release.BuildVersion }

func (m BranchManifest) FullSize() int64 {
	var size int64
	for _, a := range m.Manifest.Artifacts {
		size = max(size, a.FullSize)
	}
	return size
}

func GetBranchManifest(client *http.Client, accessToken, branch string) (BranchManifest, json.RawMessage, error) {
	var m BranchManifest
	var raw json.RawMessage
	if err := get(client, "/api/v1/ftl_integration/"+Game+"/launcher/manifest/"+url.PathEscape(branch), accessToken, &raw); err != nil {
		return m, nil, err
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, nil, err
	}
	return m, raw, nil
}

func jwtClaims(token string, out any) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	return err == nil && json.Unmarshal(payload, out) == nil
}

func JWTExpiry(token string) (time.Time, bool) {
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if !jwtClaims(token, &claims) || claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}

func GameEnv(gameToken string) []string {
	var claims struct {
		ExternalID json.RawMessage `json:"external_id"`
	}
	if !jwtClaims(gameToken, &claims) {
		return nil
	}
	id := strings.Trim(string(claims.ExternalID), `"`)
	if id == "" || strings.Trim(id, "0123456789") != "" {
		return nil
	}
	return []string{"GC_PERS_ID=" + id}
}

func LaunchTemplate(client *http.Client) string {
	data, err := httpx.Get(client, Base+"/api/v1/public_game_config/"+Game+"?WebPublishingPlatform="+Platform, 4<<20)
	if err != nil {
		return LaunchDefault
	}
	var cfg struct {
		PublicWebClientConfig struct {
			Launcher struct {
				LaunchParams string `json:"LaunchParams"`
			} `json:"Launcher"`
		} `json:"PublicWebClientConfig"`
	}
	if json.Unmarshal(data, &cfg) != nil || !strings.Contains(cfg.PublicWebClientConfig.Launcher.LaunchParams, "{token_value}") {
		return LaunchDefault
	}
	return cfg.PublicWebClientConfig.Launcher.LaunchParams
}

func LaunchArgs(template, language, languageName, email, token string) []string {
	r := strings.NewReplacer(
		"{original_command_line_param}", "",
		"{launchParams}", "",
		"{languageName}", languageName,
		"{language}", language,
		"{token_name}", "fxid-login-token",
		"{token_value}", token,
		"{email}", url.PathEscape(email),
		"{server}", "",
		"{twitch_token}", "",
	)
	return strings.Fields(r.Replace(template))
}
