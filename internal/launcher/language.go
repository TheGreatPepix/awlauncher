package launcher

import (
	"slices"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/platform"
)

var (
	vkLanguages = []string{"ru", "en"}
	fxLanguages = []string{"en", "de", "fr", "pl", "ru"}
)

var languageNames = map[string]string{
	"en": "english", "de": "german", "fr": "french", "pl": "polish", "ru": "russian",
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

func vkLanguage() string { return pickLanguage(platform.PreferredLanguages(), vkLanguages) }
func fxLanguage() string { return pickLanguage(platform.PreferredLanguages(), fxLanguages) }

func languageArgs(code string) []string {
	return []string{"-pref_language", languageNames[code]}
}
