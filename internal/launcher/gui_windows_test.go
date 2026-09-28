package launcher

import (
	"strings"
	"testing"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/ui"
)

func TestUIPageIsSelfContained(t *testing.T) {
	page, err := ui.Page()
	if err != nil {
		t.Fatal(err)
	}
	if len(page) > 2_000_000 {
		t.Fatalf("page is %d bytes", len(page))
	}
	for _, want := range []string{"<style>", "data:font/ttf;base64,", "window.aw = {", `id="page-home"`, `class="brand-logo-lid"`} {
		if !strings.Contains(page, want) {
			t.Errorf("page has no %q", want)
		}
	}
}

func TestParseHexColor(t *testing.T) {
	c, ok := parseHexColor("#fdf7ff")
	if !ok || c != [3]uint8{0xfd, 0xf7, 0xff} || colorRef(c) != 0xfff7fd {
		t.Fatalf("parseHexColor = %v %v", c, ok)
	}
	if _, ok := parseHexColor("oklch(0.5 0.1 300)"); ok {
		t.Fatal("parsed a non-hex color")
	}
}

func TestOperationsRunTogetherUnlessTheyConflict(t *testing.T) {
	g := &guiApp{}
	install := operation{Title: "Starting A", Game: true, Account: "1"}
	if reason := g.begin(&install); reason != "" {
		t.Fatal(reason)
	}
	for _, op := range []operation{{Title: "Loading branches of B", Account: "2"}, {Title: "Signing in to VK Play"}} {
		if reason := g.begin(&op); reason != "" {
			t.Fatalf("%s refused: %s", op.Title, reason)
		}
	}
	for _, op := range []operation{{Title: "Removing A", Account: "1"}, {Title: "Starting C", Game: true, Account: "3"}} {
		if g.begin(&op) == "" {
			t.Fatalf("%s started beside the install", op.Title)
		}
	}
	if !g.gameBusy() {
		t.Fatal("the install is not reported")
	}
	if left := g.finish(install.ID); left != 2 {
		t.Fatalf("%d operations left", left)
	}
	next := operation{Title: "Starting C", Game: true, Account: "3"}
	if reason := g.begin(&next); reason != "" {
		t.Fatal(reason)
	}
}

func TestQuestionShowsOnlyItsOperationsOutput(t *testing.T) {
	g := &guiApp{prompts: map[int]*pendingPrompt{}, session: &session{found: &foundGame{}}}
	install := g.opSession(operation{Title: "Starting A"})
	branches := g.opSession(operation{Title: "Loading branches of B"})
	branches.p.Say("Which branch should B play?")
	install.p.Sayf("Downloading %d%%\n", 40)
	branches.p.Say("  1  default\n  2  supertest")

	pending := func() *pendingPrompt {
		for range 200 {
			g.promptMu.Lock()
			p := g.prompts[g.nextPrompt]
			g.promptMu.Unlock()
			if p != nil {
				return p
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatal("no question")
		return nil
	}
	ask := func(s *session, want string, context ...string) {
		t.Helper()
		answer := make(chan string)
		go func() { answer <- s.p.line("Branch number: ") }()
		p := pending()
		if p.op != want || strings.Join(p.context, "|") != strings.Join(context, "|") {
			t.Fatalf("question of %q with %q", p.op, p.context)
		}
		g.answer(g.nextPrompt, "2", true)
		if got := <-answer; got != "2" {
			t.Fatalf("answer %q", got)
		}
	}
	ask(branches, "Loading branches of B", "Which branch should B play?", "  1  default", "  2  supertest")
	branches.p.Say("Not a branch number.")
	ask(branches, "Loading branches of B", "Not a branch number.")
}
