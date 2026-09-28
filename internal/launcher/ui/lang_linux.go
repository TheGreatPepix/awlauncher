package ui

import (
	"os"
	"strings"
)

func Language() string {
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(env); v != "" {
			if strings.HasPrefix(strings.ToLower(v), "ru") {
				return "ru"
			}
			return "en"
		}
	}
	return "en"
}
