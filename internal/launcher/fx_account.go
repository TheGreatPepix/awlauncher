package launcher

import (
	"errors"
	"net/http"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/fxid"
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
	req := fxid.AuthRequest{Locale: AutoLanguage(config.Account{Provider: config.ProviderFX}), EmailCredentials: &fxid.EmailCredentials{Email: email}}
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
	userID, email, err := fxid.SignedIn(resp, email)
	if err != nil {
		return config.Account{}, err
	}
	acc, err := s.cfg.SignedIn(config.Account{UserID: userID, Name: name, Provider: config.ProviderFX, Email: email})
	s.ui.Sayf("Signed in: %s", acc.Label())
	return acc, err
}

func fxLaunchArgs(client *http.Client, acc config.Account, token string) []string {
	lang := gameLanguage(acc)
	return fxid.LaunchArgs(fxid.LaunchTemplate(client), lang, languageNames[lang], acc.Email, token)
}

func fxActivateKey(client *http.Client, acc config.Account, key string) (string, error) {
	token, err := fxid.SiteToken(client, acc.UserID, gameLanguage(acc))
	if err != nil {
		return "", err
	}
	return fxid.ActivateKey(client, token, key)
}
