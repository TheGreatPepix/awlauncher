package ui

import (
	"embed"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
)

//go:embed index.html css js fonts/Rubik-Variable.ttf
var assets embed.FS

var (
	stylesheetTag = regexp.MustCompile(`<link rel="stylesheet" href="([^"]+)">`)
	scriptTag     = regexp.MustCompile(`<script src="([^"]+)"( data-demo)?></script>`)
)

func Page() (string, error) {
	html, err := assets.ReadFile("index.html")
	if err != nil {
		return "", err
	}
	font, err := assets.ReadFile("fonts/Rubik-Variable.ttf")
	if err != nil {
		return "", err
	}
	fontURL := `url("data:font/ttf;base64,` + base64.StdEncoding.EncodeToString(font) + `")`
	var missing error
	inline := func(name string) string {
		data, err := assets.ReadFile(name)
		if err != nil {
			missing = err
		}
		return string(data)
	}
	page := stylesheetTag.ReplaceAllStringFunc(string(html), func(tag string) string {
		css := inline(stylesheetTag.FindStringSubmatch(tag)[1])
		return "<style>\n" + strings.ReplaceAll(css, `url("../fonts/Rubik-Variable.ttf")`, fontURL) + "</style>"
	})
	page = scriptTag.ReplaceAllStringFunc(page, func(tag string) string {
		m := scriptTag.FindStringSubmatch(tag)
		if m[2] != "" {
			return ""
		}
		return "<script>\n" + inline(m[1]) + "</script>"
	})
	if missing != nil {
		return "", missing
	}
	if strings.Contains(page, `<script src=`) || strings.Contains(page, `rel="stylesheet"`) || strings.Contains(page, `url("../fonts/`) || strings.Contains(page, `awlauncher.ico"`) {
		return "", errors.New("the embedded page references files that were not inlined")
	}
	return page, nil
}
