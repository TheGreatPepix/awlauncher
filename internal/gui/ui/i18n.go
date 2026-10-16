package ui

import (
	"sync/atomic"

	"github.com/TheGreatPepix/awlauncher/internal/platform"
)

var guiRU = map[string]string{
	"Play": "Играть",
}

var guiLang atomic.Value

func SetLanguage(pref string) bool {
	lang := pref
	if lang != "ru" && lang != "en" {
		lang = platform.Language()
	}
	old, _ := guiLang.Load().(string)
	guiLang.Store(lang)
	return old != lang
}

func Translate(s string) string {
	if lang, _ := guiLang.Load().(string); lang == "ru" {
		if r, ok := guiRU[s]; ok {
			return r
		}
	}
	return s
}
