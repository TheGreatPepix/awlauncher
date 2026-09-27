package launcher

var getUserDefaultUILanguage = kernel32.NewProc("GetUserDefaultUILanguage")

func uiLanguage() string {
	lang, _, _ := getUserDefaultUILanguage.Call()
	if lang&0x3ff == 0x19 {
		return "ru"
	}
	return "en"
}
