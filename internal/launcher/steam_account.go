package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/fxid"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/steam"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/tokens"
)

var steamSignIn = steam.SignIn

func (s *Session) loginSteam(existing config.Account) (config.Account, error) {
	assertion, err := steamSignIn()
	if err != nil {
		return config.Account{}, err
	}
	userID, err := strconv.ParseInt(assertion.UserID, 10, 64)
	if err != nil || userID <= 0 {
		return config.Account{}, errors.New("Steam returned an invalid account ID")
	}
	if existing.UserID != 0 && userID != existing.UserID {
		return config.Account{}, errors.New("Steam is signed in to a different account")
	}
	resp, err := fxid.AuthenticateSteamOpenID(s.client, assertion.OriginURL, fxLocale(config.Account{Provider: config.ProviderSteam}))
	if err != nil {
		return config.Account{}, err
	}
	if resp.State != fxid.StateSuccess {
		return config.Account{}, resp.Failure()
	}
	if resp.Tokens == nil || resp.Tokens.GameAccessToken == "" || resp.Tokens.AccessToken == "" {
		return config.Account{}, errors.New("FX ID returned an incomplete Steam session")
	}
	if fxid.SteamID(resp.Tokens.GameAccessToken) != assertion.UserID {
		return config.Account{}, errors.New("FX ID returned a token for a different Steam account")
	}
	if err := saveSteamTokens(userID, *resp.Tokens); err != nil {
		return config.Account{}, fmt.Errorf("save Steam session: %w", err)
	}
	acc, err := s.cfg.SignedIn(config.Account{UserID: userID, Name: existing.Name, Provider: config.ProviderSteam})
	if err != nil {
		return config.Account{}, err
	}
	s.ui.Sayf("Signed in: %s", acc.Label())
	return acc, nil
}

func saveSteamTokens(userID int64, session fxid.Tokens) error {
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return tokens.Save(userID, string(data))
}

func cachedSteamTokens(userID int64) (fxid.Tokens, error) {
	data, err := tokens.Load(userID)
	if err != nil {
		return fxid.Tokens{}, err
	}
	var session fxid.Tokens
	if json.Unmarshal([]byte(data), &session) != nil || !steamTokenValid(session.GameAccessToken) || !steamTokenValid(session.AccessToken) || fxid.SteamID(session.GameAccessToken) != strconv.FormatInt(userID, 10) {
		return fxid.Tokens{}, ErrNeedLogin
	}
	return session, nil
}

func steamTokenValid(token string) bool {
	exp, ok := fxid.JWTExpiry(token)
	return ok && time.Until(exp) > 2*time.Minute
}

func (s *Session) fxTokens(acc config.Account) (fxid.Tokens, error) {
	if acc.IsSteam() {
		return cachedSteamTokens(acc.UserID)
	}
	return fxid.Session(s.client, acc.UserID, fxLocale(acc))
}

func (s *Session) siteToken(acc config.Account) (string, error) {
	t, err := s.fxTokens(acc)
	if err != nil {
		return "", err
	}
	if t.AccessToken == "" {
		return "", errors.New("FX ID returned no site access token")
	}
	return t.AccessToken, nil
}

func (s *Session) gameToken(acc config.Account) (string, error) {
	if !acc.IsSteam() {
		return fxid.GameToken(s.client, acc.UserID, fxLocale(acc))
	}
	t, err := s.fxTokens(acc)
	if err != nil {
		return "", err
	}
	if exp, ok := fxid.JWTExpiry(t.GameAccessToken); ok {
		log.Printf("Game token received, valid until %s.", exp.Local().Format("15:04 02.01.2006"))
	}
	return t.GameAccessToken, nil
}
