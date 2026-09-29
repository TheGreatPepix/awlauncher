package launcher

import (
	"slices"
	"testing"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
)

func TestPickLanguage(t *testing.T) {
	for _, c := range []struct {
		preferred []string
		supported []string
		want      string
	}{
		{[]string{"pl-PL", "en-US"}, fxLanguages, "pl"},
		{[]string{"pl-PL", "en-US"}, vkLanguages, "en"},
		{[]string{"uk-UA", "ru-RU"}, vkLanguages, "ru"},
		{[]string{"de_DE.UTF-8"}, fxLanguages, "de"},
		{[]string{"ja-JP"}, fxLanguages, "en"},
		{nil, fxLanguages, "en"},
		{[]string{"zh-CN"}, fxLanguages, "en"},
	} {
		if got := pickLanguage(c.preferred, c.supported); got != c.want {
			t.Fatalf("%q in %q: got %s, want %s", c.preferred, c.supported, got, c.want)
		}
	}
}

func TestChineseIsManualOnly(t *testing.T) {
	fx := config.Account{Provider: config.ProviderFX, Language: "zh"}
	if got := gameLanguage(fx); got != "zh" || languageNames[got] != "chineses" {
		t.Fatalf("game language %q", got)
	}
	if got := fxLocale(fx); got == "zh" {
		t.Fatal("FX ID locale must stay a supported one")
	}
	if slices.Contains(GameLanguages(config.Account{}), "zh") {
		t.Fatal("VK Play offers Chinese")
	}
}
