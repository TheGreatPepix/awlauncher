package launcher

import (
	"errors"
	"slices"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
)

var (
	vkLanguages = []string{"ru", "en"}
	fxLanguages = []string{"en", "de", "fr", "pl", "ru"}
	fxExtra     = []string{"zh"}
)

var languageNames = map[string]string{
	"en": "english", "de": "german", "fr": "french", "pl": "polish", "ru": "russian", "zh": "chineses",
}

func pickLanguage(preferred, supported []string) string {
	for _, tag := range preferred {
		code, _, _ := strings.Cut(strings.ToLower(tag), "-")
		code, _, _ = strings.Cut(code, "_")
		if slices.Contains(supported, code) {
			return code
		}
	}
	return "en"
}

func autoLanguages(acc config.Account) []string {
	if acc.IsFX() {
		return fxLanguages
	}
	return vkLanguages
}

func GameLanguages(acc config.Account) []string {
	if acc.IsFX() {
		return append(slices.Clone(fxLanguages), fxExtra...)
	}
	return vkLanguages
}

func AutoLanguage(acc config.Account) string {
	return pickLanguage(platform.PreferredLanguages(), autoLanguages(acc))
}

func fxLocale(acc config.Account) string {
	if lang := gameLanguage(acc); slices.Contains(fxLanguages, lang) {
		return lang
	}
	return AutoLanguage(acc)
}

func gameLanguage(acc config.Account) string {
	if slices.Contains(GameLanguages(acc), acc.Language) {
		return acc.Language
	}
	return AutoLanguage(acc)
}

func languageArgs(code string) []string {
	return []string{"-pref_language", languageNames[code]}
}

func (s *Session) SetGameLanguage(acc config.Account, code string) error {
	if code != "" && !slices.Contains(GameLanguages(acc), code) {
		return errors.New("this service has no such game language")
	}
	return s.cfg.UpdateAccount(acc.UserID, func(a *config.Account) { a.Language = code })
}
