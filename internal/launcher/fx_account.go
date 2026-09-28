package launcher

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/fileutil"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/fxid"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/ui"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/vkauth"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

func loginFX(p prompter, client *http.Client, cfg *config.Store, email, name string) (config.Account, error) {
	if email == "" {
		email = p.line("E-mail of the FX ID account: ")
	}
	if !strings.Contains(email, "@") {
		return config.Account{}, errors.New("not an e-mail address")
	}
	req := fxid.AuthRequest{Locale: ui.Language(), EmailCredentials: &fxid.EmailCredentials{Email: email}}
	resp, err := fxid.Authenticate(client, req)
	if err != nil {
		return config.Account{}, err
	}
	for attempt := 0; resp.State != fxid.StateSuccess; attempt++ {
		switch {
		case resp.State == fxid.StateNeedEmailCode && attempt == 0:
			p.Sayf("A code was sent to %s.\n", email)
		case resp.State == fxid.StateWrongCode && attempt < 3:
			p.Say("Wrong code.")
		default:
			return config.Account{}, resp.Failure()
		}
		code := strings.ReplaceAll(p.line("Code from the e-mail (6 digits), empty to cancel: "), " ", "")
		if code == "" {
			return config.Account{}, errors.New("sign-in cancelled")
		}
		if !fxid.ValidCode(code) {
			resp.State = fxid.StateWrongCode
			continue
		}
		req.EmailCredentials.EmailCode = &code
		if resp, err = fxid.Authenticate(client, req); err != nil {
			return config.Account{}, err
		}
	}
	if resp.Tokens == nil || resp.Tokens.RefreshToken == "" {
		return config.Account{}, errors.New("FX ID returned no session")
	}
	if resp.Email != "" {
		email = resp.Email
	}
	acc := config.Account{UserID: fxid.AccountID(email), Name: name, Provider: config.ProviderFX, Email: email}
	if err := config.SaveRefreshToken(acc.UserID, resp.Tokens.RefreshToken); err != nil {
		return config.Account{}, fmt.Errorf("save session: %w", err)
	}
	acc, err = cfg.SignedIn(acc)
	p.Sayf("Signed in: %s\n", acc.Label())
	return acc, err
}
func fxSession(client *http.Client, acc config.Account) (fxid.Tokens, error) {
	refresh, err := config.LoadRefreshToken(acc.UserID)
	if errors.Is(err, os.ErrNotExist) {
		return fxid.Tokens{}, errNeedLogin
	}
	if err != nil {
		return fxid.Tokens{}, fmt.Errorf("read session: %w", err)
	}
	resp, err := fxid.Authenticate(client, fxid.AuthRequest{Locale: ui.Language(), RefreshToken: refresh})
	if err != nil {
		var status *vkauth.HTTPStatusError
		if errors.As(err, &status) && status.Code >= 400 && status.Code < 500 {
			_ = config.ClearRefreshToken(acc.UserID)
			return fxid.Tokens{}, errNeedLogin
		}
		return fxid.Tokens{}, fmt.Errorf("refresh session: %w", err)
	}
	if resp.State != fxid.StateSuccess || resp.Tokens == nil || resp.Tokens.GameAccessToken == "" {
		_ = config.ClearRefreshToken(acc.UserID)
		return fxid.Tokens{}, errNeedLogin
	}
	if resp.Tokens.RefreshToken != "" && resp.Tokens.RefreshToken != refresh {
		if err := config.SaveRefreshToken(acc.UserID, resp.Tokens.RefreshToken); err != nil {
			return fxid.Tokens{}, fmt.Errorf("save session: %w", err)
		}
	}
	return *resp.Tokens, nil
}
func fxSiteToken(client *http.Client, acc config.Account) (string, error) {
	tokens, err := fxSession(client, acc)
	if err != nil {
		return "", err
	}
	if tokens.AccessToken == "" {
		return "", errors.New("FX ID returned no site access token")
	}
	return tokens.AccessToken, nil
}
func fxGameToken(client *http.Client, acc config.Account) (string, error) {
	tokens, err := fxSession(client, acc)
	if err != nil {
		return "", err
	}
	if exp, ok := fxid.JWTExpiry(tokens.GameAccessToken); ok {
		fmt.Printf("Game token received, valid until %s.\n", exp.Local().Format("15:04 02.01.2006"))
	}
	return tokens.GameAccessToken, nil
}
func fxLaunchArgs(client *http.Client, acc config.Account, token string) []string {
	return fxid.LaunchArgs(fxid.LaunchTemplate(client), ui.Language(), acc.Email, token)
}
func fxActivateKey(client *http.Client, acc config.Account, key string) (string, error) {
	token, err := fxSiteToken(client, acc)
	if err != nil {
		return "", err
	}
	return fxid.ActivateKey(client, token, key)
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

func saveBranchManifest(name string, raw []byte) string {
	if !fxid.ValidBranchName(name) {
		return ""
	}
	dir, err := platform.DataDir()
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
func fxBranches(client *http.Client, acc config.Account) ([]fxBranchInfo, error) {
	token, err := fxSiteToken(client, acc)
	if err != nil {
		return nil, err
	}
	names, err := fxid.AvailableBranches(client, token)
	if err != nil {
		return nil, fmt.Errorf("branch list: %w", err)
	}
	var out []fxBranchInfo
	for _, name := range names {
		info := fxBranchInfo{Name: name}
		if m, raw, err := fxid.GetBranchManifest(client, token, name); err != nil {
			info.Err = err
		} else {
			info.Saved = saveBranchManifest(name, raw)
			info.Version = m.Manifest.Release.BuildVersion
			info.Build = m.Manifest.Release.BuildNumber
			info.CreatedAt = m.Manifest.Release.CreatedAt
			info.FullSize = m.FullSize()
		}
		out = append(out, info)
	}
	return out, nil
}
func printFXBranches(p prompter, branches []fxBranchInfo) {
	if len(branches) == 0 {
		p.Say("No branches are available to this account.")
		return
	}
	width := 0
	for _, b := range branches {
		width = max(width, len(b.Name))
	}
	p.Say("FX ID client branches available to this account:")
	for _, b := range branches {
		if b.Err != nil {
			p.Sayf("  %-*s  manifest: %v\n", width, b.Name, b.Err)
			continue
		}
		line := fmt.Sprintf("  %-*s  %s (build %d)", width, b.Name, b.Version, b.Build)
		if !b.CreatedAt.IsZero() {
			line += ", " + b.CreatedAt.Local().Format("02.01.2006")
		}
		if b.FullSize > 0 {
			line += ", " + progress.FormatBytes(b.FullSize)
		}
		p.Say(line)
	}
	for _, b := range branches {
		if b.Saved != "" {
			p.Say("Branch manifests saved to", filepath.Dir(b.Saved))
			return
		}
	}
}
