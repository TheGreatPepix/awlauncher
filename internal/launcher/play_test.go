package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
)

func TestPickPlayAccount(t *testing.T) {
	cfg := config.Config{
		Accounts:   []config.Account{{UserID: 10, Name: "Tanker"}, {UserID: 20, Name: "EU main", Provider: config.ProviderFX, Email: "player@example.com"}},
		LastUserID: 20,
	}
	for sel, want := range map[string]int64{"": 20, "1": 10, "2": 20, "tanker": 10, "EU MAIN": 20, "player@example.com": 20, "10": 10} {
		acc, err := pickPlayAccount(cfg, sel)
		if err != nil || acc.UserID != want {
			t.Errorf("pickPlayAccount(%q) = %d, %v; want %d", sel, acc.UserID, err, want)
		}
	}
	for _, sel := range []string{"0", "3", "nobody"} {
		if _, err := pickPlayAccount(cfg, sel); err == nil {
			t.Errorf("pickPlayAccount(%q) found an account", sel)
		}
	}
	if _, err := pickPlayAccount(config.Config{}, ""); err == nil {
		t.Error("an account was picked from an empty list")
	}
}

func TestUnattendedPrompter(t *testing.T) {
	p := unattendedPrompter()
	if !p.Yes("Install now?", true) || p.Yes("Start the game without updating?", false) || !p.Yes("Download and repair these files now?", true) {
		t.Error("unattended answers do not follow the defaults")
	}
	if p.Yes(`Install Armored Warfare into /home/deck/Games/Armored Warfare?`, true) || p.Yes("The game is not installed in /games (unfinished install?). Install it there?", true) {
		t.Error("a new install is accepted unattended")
	}
	if p.line("Game folder, empty to quit: ") != "" {
		t.Error("an unattended question got an answer")
	}
}

func TestUnattendedDeclinesMatchQuestions(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := filepath.Glob(filepath.Join("gamefiles", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, domain...)
	var source strings.Builder
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || f == "play.go" {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		source.Write(data)
	}
	for _, prefix := range unattendedDeclines {
		if !strings.Contains(source.String(), `"`+prefix) {
			t.Errorf("no question starts with %q", prefix)
		}
	}
}
