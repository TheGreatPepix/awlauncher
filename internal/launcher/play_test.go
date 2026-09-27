package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPickPlayAccount(t *testing.T) {
	cfg := launcherConfig{
		Accounts:   []account{{UserID: 10, Name: "Tanker"}, {UserID: 20, Name: "EU main", Provider: providerFX, Email: "player@example.com"}},
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
	if _, err := pickPlayAccount(launcherConfig{}, ""); err == nil {
		t.Error("an account was picked from an empty list")
	}
}

func TestUnattendedPrompter(t *testing.T) {
	p := unattendedPrompter()
	if !p.yes("Install now?", true) || p.yes("Start the game without updating?", false) || !p.yes("Download and repair these files now?", true) {
		t.Error("unattended answers do not follow the defaults")
	}
	if p.yes(`Install Armored Warfare into /home/deck/Games/Armored Warfare?`, true) || p.yes("The game is not installed in /games (unfinished install?). Install it there?", true) {
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
