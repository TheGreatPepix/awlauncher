package ui

import (
	"sync/atomic"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
)

var guiRU = map[string]string{
	"Play":                           "Играть",
	"Start the last played account":  "Запустить игру за основной аккаунт",
	"Open AWLauncher":                "Открыть AWLauncher",
	"Show the launcher":              "Показать лаунчер",
	"Exit":                           "Выход",
	"Close the launcher":             "Закрыть лаунчер",
	"AWLauncher is already running.": "AWLauncher уже запущен.",
	"Cannot create the tray icon: ":  "Не удалось создать значок в области уведомлений: ",
	"AWLauncher needs the Microsoft Edge WebView2 Runtime, which is part of Windows 11 and current Windows 10.\n\nOpen the download page?": "AWLauncher нужен Microsoft Edge WebView2 Runtime, который входит в Windows 11 и актуальную Windows 10.\n\nОткрыть страницу загрузки?",
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
