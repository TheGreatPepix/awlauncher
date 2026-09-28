package launcher

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/fileutil"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/fxid"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/vkauth"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

func (s *Session) loginFX(email, name string) (config.Account, error) {
	if email == "" {
		answer, ok := s.ui.Ask(Prompt{Kind: PromptEmail, Question: "E-mail of the FX ID account"})
		if !ok {
			return config.Account{}, ErrCancelled
		}
		email = strings.TrimSpace(answer)
	}
	if !strings.Contains(email, "@") {
		return config.Account{}, errors.New("not an e-mail address")
	}
	req := fxid.AuthRequest{Locale: platform.Language(), EmailCredentials: &fxid.EmailCredentials{Email: email}}
	resp, err := fxid.Authenticate(s.client, req)
	if err != nil {
		return config.Account{}, err
	}
	for attempt := 0; resp.State != fxid.StateSuccess; attempt++ {
		switch {
		case resp.State == fxid.StateNeedEmailCode && attempt == 0:
			s.ui.Sayf("A code was sent to %s.", email)
		case resp.State == fxid.StateWrongCode && attempt < 3:
			s.ui.Say("Wrong code.")
		default:
			return config.Account{}, resp.Failure()
		}
		answer, ok := s.ui.Ask(Prompt{Kind: PromptCode, Question: "Code from the e-mail"})
		code := strings.ReplaceAll(answer, " ", "")
		if !ok || code == "" {
			return config.Account{}, ErrCancelled
		}
		if !fxid.ValidCode(code) {
			resp.State = fxid.StateWrongCode
			continue
		}
		req.EmailCredentials.EmailCode = &code
		if resp, err = fxid.Authenticate(s.client, req); err != nil {
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
	acc, err = s.cfg.SignedIn(acc)
	s.ui.Sayf("Signed in: %s", acc.Label())
	return acc, err
}

func fxSession(client *http.Client, acc config.Account) (fxid.Tokens, error) {
	refresh, err := config.LoadRefreshToken(acc.UserID)
	if errors.Is(err, os.ErrNotExist) {
		return fxid.Tokens{}, ErrNeedLogin
	}
	if err != nil {
		return fxid.Tokens{}, fmt.Errorf("read session: %w", err)
	}
	resp, err := fxid.Authenticate(client, fxid.AuthRequest{Locale: platform.Language(), RefreshToken: refresh})
	if err != nil {
		var status *vkauth.HTTPStatusError
		if errors.As(err, &status) && status.Code >= 400 && status.Code < 500 {
			_ = config.ClearRefreshToken(acc.UserID)
			return fxid.Tokens{}, ErrNeedLogin
		}
		return fxid.Tokens{}, fmt.Errorf("refresh session: %w", err)
	}
	if resp.State != fxid.StateSuccess || resp.Tokens == nil || resp.Tokens.GameAccessToken == "" {
		_ = config.ClearRefreshToken(acc.UserID)
		return fxid.Tokens{}, ErrNeedLogin
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
		log.Printf("Game token received, valid until %s.", exp.Local().Format("15:04 02.01.2006"))
	}
	return tokens.GameAccessToken, nil
}
func fxLaunchArgs(client *http.Client, acc config.Account, token string) []string {
	return fxid.LaunchArgs(fxid.LaunchTemplate(client), platform.Language(), acc.Email, token)
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
func logFXBranches(branches []fxBranchInfo) {
	for _, b := range branches {
		if b.Err != nil {
			log.Printf("  %s  manifest: %v", b.Name, b.Err)
			continue
		}
		log.Printf("  %s  %s", b.Name, b.describe())
	}
	for _, b := range branches {
		if b.Saved != "" {
			log.Print("Branch manifests saved to ", filepath.Dir(b.Saved))
			return
		}
	}
}

func (b fxBranchInfo) describe() string {
	line := fmt.Sprintf("%s (build %d)", b.Version, b.Build)
	if !b.CreatedAt.IsZero() {
		line += ", " + b.CreatedAt.Local().Format("02.01.2006")
	}
	if b.FullSize > 0 {
		line += ", " + progress.FormatBytes(b.FullSize)
	}
	return line
}
