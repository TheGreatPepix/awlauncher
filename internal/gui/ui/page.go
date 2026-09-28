package ui

import (
	"embed"
	"encoding/base64"
	"errors"
	"strings"
)

//go:embed index.html app.css app.js i18n.js fonts/Rubik-Variable.ttf
var assets embed.FS

func Page() (string, error) {
	html, err := assets.ReadFile("index.html")
	if err != nil {
		return "", err
	}
	css, err := assets.ReadFile("app.css")
	if err != nil {
		return "", err
	}
	js, err := assets.ReadFile("app.js")
	if err != nil {
		return "", err
	}
	i18nJS, err := assets.ReadFile("i18n.js")
	if err != nil {
		return "", err
	}
	font, err := assets.ReadFile("fonts/Rubik-Variable.ttf")
	if err != nil {
		return "", err
	}
	style := strings.Replace(string(css), `url("fonts/Rubik-Variable.ttf")`, `url("data:font/ttf;base64,`+base64.StdEncoding.EncodeToString(font)+`")`, 1)
	page := strings.Replace(string(html), `<link rel="stylesheet" href="app.css">`, "<style>\n"+style+"</style>", 1)
	page = strings.Replace(page, `<script src="i18n.js"></script>`, "<script>\n"+string(i18nJS)+"</script>", 1)
	page = strings.Replace(page, `<script src="app.js"></script>`, "<script>\n"+string(js)+"</script>", 1)
	if strings.Contains(page, `href="app.css"`) || strings.Contains(page, `src="app.js"`) || strings.Contains(page, `src="i18n.js"`) || strings.Contains(page, `awlauncher.ico"`) || strings.Contains(page, `url("fonts/`) {
		return "", errors.New("the embedded page references files that were not inlined")
	}
	return page, nil
}
