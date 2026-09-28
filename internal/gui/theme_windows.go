package gui

import "golang.org/x/sys/windows/registry"

func themeColors(theme string) (dark bool, surface, text [3]uint8) {
	if theme == "dark" || (theme != "light" && systemDarkTheme()) {
		return true, [3]uint8{0x14, 0x12, 0x18}, [3]uint8{0xE6, 0xE0, 0xE9}
	}
	return false, [3]uint8{0xFD, 0xF7, 0xFF}, [3]uint8{0x1D, 0x1B, 0x20}
}

func systemDarkTheme() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && v == 0
}

func colorRef(c [3]uint8) uint32 { return uint32(c[0]) | uint32(c[1])<<8 | uint32(c[2])<<16 }
