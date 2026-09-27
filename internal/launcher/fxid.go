package launcher

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TheGreatPepix/awlauncher/internal/fileutil"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var fxBase = "https://id.wishlistgames.net"

const (
	fxGame          = "aw"
	fxPlatform      = "11"
	providerFX      = "fxid"
	fxLaunchDefault = "{original_command_line_param} {launchParams} -pref_language {languageName} --{token_name}={token_value}"
)

const (
	fxStateNeedEmailCode = 3
	fxStateWrongCode     = 4
	fxStateSuccess       = 99
)

var fxStateNames = map[int]string{
	1: "no credentials", 2: "wrong e-mail for this account", 5: "unknown server error",
	6: "validation error", 7: "invite error", 9: "account has no e-mail", 10: "account has no e-mail",
	11: "too many requests, try again later", 12: "e-mail is already taken", 13: "account is banned",
}

var fxCodePattern = regexp.MustCompile(`^[0-9]{6}$`)

type fxEmailCredentials struct {
	Email     string  `json:"Email"`
	EmailCode *string `json:"EmailCode"`
}

type fxAuthRequest struct {
	GameSystemName    string `json:"GameSystemName"`
	Locale            string `json:"Locale"`
	SessionID         string `json:"SessionId"`
	RefreshToken      string `json:"RefreshToken,omitempty"`
	AdditionalOptions struct {
		AllowOnlyNewEmail bool `json:"AllowOnlyNewEmail"`
	} `json:"AdditionalOptions"`
	EmailCredentials *fxEmailCredentials `json:"EmailCredentials,omitempty"`
}

type fxTokens struct {
	RefreshToken    string `json:"RefreshToken"`
	GameAccessToken string `json:"GameAccessToken"`
	AccessToken     string `json:"AccessToken"`
}

type fxAuthResponse struct {
	State  int       `json:"State"`
	Tokens *fxTokens `json:"Tokens"`
	Hint   string    `json:"Hint"`
	Email  string    `json:"Email"`
}

func (r fxAuthResponse) failure() error {
	name := fxStateNames[r.State]
	if name == "" {
		name = fmt.Sprintf("state %d", r.State)
	}
	if r.Hint != "" {
		return fmt.Errorf("FX ID: %s (%s)", name, r.Hint)
	}
	return fmt.Errorf("FX ID: %s", name)
}

func fxAuthenticate(client *http.Client, req fxAuthRequest) (fxAuthResponse, error) {
	var resp fxAuthResponse
	req.GameSystemName = fxGame
	req.Locale = uiLanguage()
	body, err := json.Marshal(req)
	if err != nil {
		return resp, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, fxBase+"/api/v1/auth/authenticate_with_external_id/"+fxGame, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", browserAgent)
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
		return resp, &httpStatusError{Host: httpReq.URL.Host, Code: r.StatusCode}
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.New("unexpected FX ID response")
	}
	return resp, nil
}

func fxAccountID(email string) int64 {
	h := fnv.New64a()
	h.Write([]byte(strings.ToLower(strings.TrimSpace(email))))
	return -int64(h.Sum64() >> 1)
}

func loginFX(p prompter, client *http.Client, cfg *configStore, email, name string) (account, error) {
	if email == "" {
		email = p.line("E-mail of the FX ID account: ")
	}
	if !strings.Contains(email, "@") {
		return account{}, errors.New("not an e-mail address")
	}
	req := fxAuthRequest{EmailCredentials: &fxEmailCredentials{Email: email}}
	resp, err := fxAuthenticate(client, req)
	if err != nil {
		return account{}, err
	}
	for attempt := 0; resp.State != fxStateSuccess; attempt++ {
		switch {
		case resp.State == fxStateNeedEmailCode && attempt == 0:
			p.sayf("A code was sent to %s.\n", email)
		case resp.State == fxStateWrongCode && attempt < 3:
			p.say("Wrong code.")
		default:
			return account{}, resp.failure()
		}
		code := strings.ReplaceAll(p.line("Code from the e-mail (6 digits), empty to cancel: "), " ", "")
		if code == "" {
			return account{}, errors.New("sign-in cancelled")
		}
		if !fxCodePattern.MatchString(code) {
			resp.State = fxStateWrongCode
			continue
		}
		req.EmailCredentials.EmailCode = &code
		if resp, err = fxAuthenticate(client, req); err != nil {
			return account{}, err
		}
	}
	if resp.Tokens == nil || resp.Tokens.RefreshToken == "" {
		return account{}, errors.New("FX ID returned no session")
	}
	if resp.Email != "" {
		email = resp.Email
	}
	acc := account{UserID: fxAccountID(email), Name: name, Provider: providerFX, Email: email}
	if err := saveRefreshToken(acc.UserID, resp.Tokens.RefreshToken); err != nil {
		return account{}, fmt.Errorf("save session: %w", err)
	}
	acc, err = cfg.signedIn(acc)
	p.sayf("Signed in: %s\n", acc.label())
	return acc, err
}

func fxSession(client *http.Client, acc account) (fxTokens, error) {
	refresh, err := loadRefreshToken(acc.UserID)
	if errors.Is(err, os.ErrNotExist) {
		return fxTokens{}, errNeedLogin
	}
	if err != nil {
		return fxTokens{}, fmt.Errorf("read session: %w", err)
	}
	resp, err := fxAuthenticate(client, fxAuthRequest{RefreshToken: refresh})
	if err != nil {
		var status *httpStatusError
		if errors.As(err, &status) && status.Code >= 400 && status.Code < 500 {
			_ = clearRefreshToken(acc.UserID)
			return fxTokens{}, errNeedLogin
		}
		return fxTokens{}, fmt.Errorf("refresh session: %w", err)
	}
	if resp.State != fxStateSuccess || resp.Tokens == nil || resp.Tokens.GameAccessToken == "" {
		_ = clearRefreshToken(acc.UserID)
		return fxTokens{}, errNeedLogin
	}
	if resp.Tokens.RefreshToken != "" && resp.Tokens.RefreshToken != refresh {
		if err := saveRefreshToken(acc.UserID, resp.Tokens.RefreshToken); err != nil {
			return fxTokens{}, fmt.Errorf("save session: %w", err)
		}
	}
	return *resp.Tokens, nil
}

func fxGameToken(client *http.Client, acc account) (string, error) {
	tokens, err := fxSession(client, acc)
	if err != nil {
		return "", err
	}
	if exp, ok := jwtExpiry(tokens.GameAccessToken); ok {
		fmt.Printf("Game token received, valid until %s.\n", exp.Local().Format("15:04 02.01.2006"))
	}
	return tokens.GameAccessToken, nil
}

var errFXNoAccess = errors.New("no access")

func fxAPI(client *http.Client, method, path, accessToken string, body, out any) error {
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
	req, err := http.NewRequest(method, fxBase+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", browserAgent)
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
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden,
		resp.StatusCode >= 300 && resp.StatusCode < 400:
		return fmt.Errorf("%w (HTTP %d)", errFXNoAccess, resp.StatusCode)
	default:
		if msg := problemText(data); msg != "" {
			return fmt.Errorf("%s (HTTP %d)", msg, resp.StatusCode)
		}
		return &httpStatusError{Host: req.URL.Host, Code: resp.StatusCode}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

func fxAPIGet(client *http.Client, path, accessToken string, out any) error {
	return fxAPI(client, http.MethodGet, path, accessToken, nil, out)
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

func fxActivateKey(client *http.Client, acc account, key string) (string, error) {
	tokens, err := fxSession(client, acc)
	if err != nil {
		return "", err
	}
	if tokens.AccessToken == "" {
		return "", errors.New("FX ID returned no site access token")
	}
	var resp struct {
		ActivatedBranch string `json:"ActivatedBranch"`
	}
	body := struct {
		Key string `json:"Key"`
	}{Key: key}
	if err := fxAPI(client, http.MethodPost, "/api/v1/ftl_integration/"+fxGame+"/launcher/activate_key", tokens.AccessToken, body, &resp); err != nil {
		return "", err
	}
	if resp.ActivatedBranch == "" {
		return "", errors.New("the key was not accepted")
	}
	return resp.ActivatedBranch, nil
}

type fxBranchInfo struct {
	Name      string
	Version   string
	Build     int64
	CreatedAt time.Time
	FullSize  int64
	Saved     string
	Err       error
}

var branchNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func saveBranchManifest(name string, raw []byte) string {
	if !branchNamePattern.MatchString(name) {
		return ""
	}
	dir, err := launcherDir()
	if err != nil {
		return ""
	}
	dir = filepath.Join(dir, "branches")
	if os.MkdirAll(dir, 0700) != nil {
		return ""
	}
	path := filepath.Join(dir, name+".json")
	if fileutil.WriteAtomic(path, raw) != nil {
		return ""
	}
	return path
}

func fxBranches(client *http.Client, acc account) ([]fxBranchInfo, error) {
	tokens, err := fxSession(client, acc)
	if err != nil {
		return nil, err
	}
	if tokens.AccessToken == "" {
		return nil, errors.New("FX ID returned no site access token")
	}
	var list struct {
		Branches []struct {
			Name string `json:"Name"`
		} `json:"Branches"`
	}
	if err := fxAPIGet(client, "/api/v1/ftl_integration/"+fxGame+"/launcher/available_branches", tokens.AccessToken, &list); err != nil {
		return nil, fmt.Errorf("branch list: %w", err)
	}
	var out []fxBranchInfo
	for _, b := range list.Branches {
		info := fxBranchInfo{Name: b.Name}
		if m, raw, err := fxGetBranchManifest(client, tokens.AccessToken, b.Name); err != nil {
			info.Err = err
		} else {
			info.Saved = saveBranchManifest(b.Name, raw)
			info.Version = m.Manifest.Release.BuildVersion
			info.Build = m.Manifest.Release.BuildNumber
			info.CreatedAt = m.Manifest.Release.CreatedAt
			for _, a := range m.Manifest.Artifacts {
				info.FullSize = max(info.FullSize, a.FullSize)
			}
		}
		out = append(out, info)
	}
	return out, nil
}

func printFXBranches(p prompter, branches []fxBranchInfo) {
	if len(branches) == 0 {
		p.say("No branches are available to this account.")
		return
	}
	width := 0
	for _, b := range branches {
		width = max(width, len(b.Name))
	}
	p.say("FX ID client branches available to this account:")
	for _, b := range branches {
		if b.Err != nil {
			p.sayf("  %-*s  manifest: %v\n", width, b.Name, b.Err)
			continue
		}
		line := fmt.Sprintf("  %-*s  %s (build %d)", width, b.Name, b.Version, b.Build)
		if !b.CreatedAt.IsZero() {
			line += ", " + b.CreatedAt.Local().Format("02.01.2006")
		}
		if b.FullSize > 0 {
			line += ", " + progress.FormatBytes(b.FullSize)
		}
		p.say(line)
	}
	for _, b := range branches {
		if b.Saved != "" {
			p.say("Branch manifests saved to", filepath.Dir(b.Saved))
			return
		}
	}
}

func jwtExpiry(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}

func fxLaunchTemplate(client *http.Client) string {
	data, err := fetch(client, fxBase+"/api/v1/public_game_config/"+fxGame+"?WebPublishingPlatform="+fxPlatform, 4<<20)
	if err != nil {
		return fxLaunchDefault
	}
	var cfg struct {
		PublicWebClientConfig struct {
			Launcher struct {
				LaunchParams string `json:"LaunchParams"`
			} `json:"Launcher"`
		} `json:"PublicWebClientConfig"`
	}
	if json.Unmarshal(data, &cfg) != nil || !strings.Contains(cfg.PublicWebClientConfig.Launcher.LaunchParams, "{token_value}") {
		return fxLaunchDefault
	}
	return cfg.PublicWebClientConfig.Launcher.LaunchParams
}

func fxLaunchArgs(template string, acc account, token string) []string {
	language := map[string]string{"ru": "russian"}[uiLanguage()]
	if language == "" {
		language = "english"
	}
	r := strings.NewReplacer(
		"{original_command_line_param}", "",
		"{launchParams}", "",
		"{languageName}", language,
		"{language}", uiLanguage(),
		"{token_name}", "fxid-login-token",
		"{token_value}", token,
		"{email}", url.PathEscape(acc.Email),
		"{server}", "",
		"{twitch_token}", "",
	)
	return strings.Fields(r.Replace(template))
}
