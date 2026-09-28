package ui

import "golang.org/x/sys/windows"

var getUserDefaultUILanguage = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")

func Language() string {
	lang, _, _ := getUserDefaultUILanguage.Call()
	if lang&0x3ff == 0x19 {
		return "ru"
	}
	return "en"
}
