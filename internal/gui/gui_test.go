package gui

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/gui/ui"
	"github.com/TheGreatPepix/awlauncher/internal/launcher"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
)

type fakeHost struct {
	mu     sync.Mutex
	events []map[string]any
}

func (h *fakeHost) post(f func()) { f() }

func (h *fakeHost) eval(script string) {
	data := strings.TrimSuffix(strings.TrimPrefix(script, "window.aw && aw.recv("), ")")
	var event map[string]any
	if json.Unmarshal([]byte(data), &event) == nil {
		h.mu.Lock()
		h.events = append(h.events, event)
		h.mu.Unlock()
	}
}

func (h *fakeHost) prompts() []map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []map[string]any
	for _, e := range h.events {
		if e["type"] == "prompt" {
			out = append(out, e)
		}
	}
	return out
}

func (h *fakeHost) showWindow()                                           {}
func (h *fakeHost) hideWindow()                                           {}
func (h *fakeHost) exit()                                                 {}
func (h *fakeHost) setTitleBar(bool, [3]uint8, [3]uint8)                  {}
func (h *fakeHost) pickFolder(string) (string, error)                     { return "", nil }
func (h *fakeHost) copyText(string) error                                 { return nil }
func (h *fakeHost) open(string)                                           {}
func (h *fakeHost) setTrayPlay(string, bool)                              {}
func (h *fakeHost) relabelTray()                                          {}
func (h *fakeHost) openSignIn(string, func(string, string), func()) error { return nil }
func (h *fakeHost) showSignIn()                                           {}
func (h *fakeHost) closeSignIn()                                          {}

func testApp(t *testing.T) (*App, *fakeHost) {
	t.Helper()
	dataDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dataDir)
	t.Setenv("XDG_DATA_HOME", dataDir)
	h := &fakeHost{}
	logs := &logBuffer{}
	log.SetOutput(logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	g := newApp(config.NewStore(config.Config{}), logs, false)
	g.host = h
	return g, h
}

func waitPrompt(t *testing.T, h *fakeHost, n int) map[string]any {
	t.Helper()
	for range 200 {
		if p := h.prompts(); len(p) >= n {
			return p[n-1]
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("no question")
	return nil
}

func TestUIPageIsSelfContained(t *testing.T) {
	page, err := ui.Page()
	if err != nil {
		t.Fatal(err)
	}
	if len(page) > 2_000_000 {
		t.Fatalf("page is %d bytes", len(page))
	}
	for _, want := range []string{"<style>", "data:font/ttf;base64,", "window.aw = {", `id="page-home"`, `class="brand-logo-lid"`, "--spring:", "function setupPlayShapes"} {
		if !strings.Contains(page, want) {
			t.Errorf("page has no %q", want)
		}
	}
	for _, unwanted := range []string{"const demo", "dev/demo.js", "<script src=", `rel="stylesheet"`} {
		if strings.Contains(page, unwanted) {
			t.Errorf("page still has %q", unwanted)
		}
	}
}

func TestParseHexColor(t *testing.T) {
	c, ok := parseHexColor("#fdf7ff")
	if !ok || c != [3]uint8{0xfd, 0xf7, 0xff} {
		t.Fatalf("parseHexColor = %v %v", c, ok)
	}
	if _, ok := parseHexColor("oklch(0.5 0.1 300)"); ok {
		t.Fatal("parsed a non-hex color")
	}
}

func TestQuestionShowsOnlyItsOperationsOutput(t *testing.T) {
	g, h := testApp(t)
	g.pageReady = true
	install := g.opUI("Starting A")
	branches := g.opUI("Loading branches of B")
	branches.Say("Asking FX ID for branches of B...")
	install.Sayf("Downloading %d%%\n", 40)
	branches.Say("Wrong code.\nTry again.")

	answer := make(chan string)
	go func() {
		v, _ := branches.Ask(launcher.Prompt{Kind: launcher.PromptChoice, Question: "Which branch should B play?", Options: []launcher.Choice{{Value: "supertest", Label: "supertest"}}})
		answer <- v
	}()
	p := waitPrompt(t, h, 1)
	context, _ := json.Marshal(p["context"])
	if p["op"] != "Loading branches of B" || p["kind"] != "choice" || string(context) != `["Asking FX ID for branches of B...","Wrong code.","Try again."]` {
		t.Fatalf("prompt = %v", p)
	}
	if options, _ := p["options"].([]any); len(options) != 1 {
		t.Fatalf("options = %v", p["options"])
	}
	g.answer(int(p["id"].(float64)), " supertest ", true)
	if got := <-answer; got != "supertest" {
		t.Fatalf("answer %q", got)
	}
	if !strings.Contains(g.logs.String(), "Downloading 40%") {
		t.Fatalf("log = %q", g.logs.String())
	}
}

func TestConfirmAndFolderPromptsAreTyped(t *testing.T) {
	g, h := testApp(t)
	g.pageReady = true
	u := g.opUI("Downloading game")
	yes := make(chan bool)
	go func() { yes <- u.Yes("Install now?", true) }()
	p := waitPrompt(t, h, 1)
	if p["kind"] != "confirm" || p["default"] != true || p["question"] != "Install now?" {
		t.Fatalf("confirm = %v", p)
	}
	g.answer(int(p["id"].(float64)), "yes", true)
	if !<-yes {
		t.Fatal("yes was not accepted")
	}
	go u.Ask(g.session.FolderPrompt(gamefiles.KindBranch, "SuperTest"))
	p = waitPrompt(t, h, 2)
	if p["kind"] != "folder" || p["folder"] != "branch" || p["branch"] != "SuperTest" || p["suggest"] == "" {
		t.Fatalf("folder = %v", p)
	}
	g.answer(int(p["id"].(float64)), "", false)
}

func TestReadySendsWaitingPrompts(t *testing.T) {
	g, h := testApp(t)
	go g.opUI("Starting A").Yes("Install now?", true)
	for range 200 {
		g.promptMu.Lock()
		n := len(g.prompts)
		g.promptMu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(h.prompts()) != 0 {
		t.Fatal("a prompt reached a page that is not loaded")
	}
	g.onMessage(`{"cmd":"ready"}`)
	p := waitPrompt(t, h, 1)
	g.answer(int(p["id"].(float64)), "no", true)
}
