package gui

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
)

type fakeHost struct {
	mu     sync.Mutex
	events []map[string]any
	shown  int
	exited int
}

func (h *fakeHost) post(f func()) { f() }

func (h *fakeHost) send(data []byte) {
	var event map[string]any
	if json.Unmarshal(data, &event) == nil {
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

func (h *fakeHost) showWindow()              { h.shown++ }
func (h *fakeHost) hideWindow()              {}
func (h *fakeHost) windowShown() bool        { return true }
func (h *fakeHost) exit()                    { h.exited++ }
func (h *fakeHost) open(string)              {}
func (h *fakeHost) setTrayPlay(string, bool) {}

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

func TestQuestionShowsOnlyItsOperationsOutput(t *testing.T) {
	g, h := testApp(t)
	g.pageReady.Store(true)
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
	g.pageReady.Store(true)
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

func TestPrefsKeepPalettes(t *testing.T) {
	want := uiPrefs{Theme: "dark", Hue: 150, Style: "expressive", Palettes: []uiPalette{{Hue: 150, Style: "expressive"}, {Hue: 20, Style: "vibrant"}}, Lang: "ru"}
	if err := savePrefs(want); err != nil {
		t.Fatal(err)
	}
	got := loadPrefs()
	if got.Theme != want.Theme || got.Hue != want.Hue || got.Style != want.Style || got.Lang != want.Lang || len(got.Palettes) != 2 || got.Palettes[1] != want.Palettes[1] {
		t.Fatalf("prefs = %+v", got)
	}
}

func TestTrayExitAsksWhileAnOperationRuns(t *testing.T) {
	g, h := testApp(t)
	g.pageReady.Store(true)
	op := launcher.Operation{Title: "Downloading game", Game: true}
	g.ops.Begin(&op)
	g.requestExit()
	if h.exited != 0 || h.shown != 1 {
		t.Fatalf("exited %d, shown %d; want the window shown and no exit", h.exited, h.shown)
	}
	if n := len(h.events); n == 0 || h.events[n-1]["type"] != "confirmExit" {
		t.Fatalf("events %v; want confirmExit last", h.events)
	}
	g.ops.Finish(op.ID)
	g.requestExit()
	if h.exited != 1 {
		t.Fatalf("exited %d; want an exit once nothing runs", h.exited)
	}
}
