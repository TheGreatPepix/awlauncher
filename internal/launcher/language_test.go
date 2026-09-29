package launcher

import "testing"

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
	} {
		if got := pickLanguage(c.preferred, c.supported); got != c.want {
			t.Fatalf("%q in %q: got %s, want %s", c.preferred, c.supported, got, c.want)
		}
	}
}
