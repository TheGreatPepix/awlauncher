package gui

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/TheGreatPepix/awlauncher/internal/launcher"
)

const (
	promptConfirm launcher.PromptKind = "confirm"
	contextLines                      = 40
)

type opUI struct {
	g     *App
	title string
	out   opOutput
}

func (g *App) opUI(title string) *opUI { return &opUI{g: g, title: title} }

func (u *opUI) Say(a ...any) { u.tell(strings.TrimSuffix(fmt.Sprintln(a...), "\n")) }

func (u *opUI) Sayf(format string, a ...any) {
	u.tell(strings.TrimSuffix(fmt.Sprintf(format, a...), "\n"))
}

func (u *opUI) tell(text string) {
	log.Print(text)
	u.out.note(text)
}

func (u *opUI) Notify(text string) {
	u.tell(text + ".")
	u.g.emit(map[string]any{"type": "notice", "message": text, "kind": "ok"})
}

func (u *opUI) Yes(question string, def bool) bool {
	r := u.g.ask(launcher.Prompt{Kind: promptConfirm, Question: question}, def, u.title, u.out.take())
	return r.ok && r.value == "yes"
}

func (u *opUI) Ask(p launcher.Prompt) (string, bool) {
	r := u.g.ask(p, false, u.title, u.out.take())
	return strings.TrimSpace(r.value), r.ok
}

func (u *opUI) VKSignIn(s launcher.VKSignIn) func() { return u.g.vkSignInStarted(s) }

func (u *opUI) Foreground() { u.g.host.post(u.g.host.showWindow) }

type opOutput struct {
	mu    sync.Mutex
	lines []string
}

func (o *opOutput) note(text string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimRight(line, "\r"); strings.TrimSpace(line) != "" {
			o.lines = append(o.lines, line)
		}
	}
	if len(o.lines) > contextLines {
		o.lines = o.lines[len(o.lines)-contextLines:]
	}
}

func (o *opOutput) take() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	lines := o.lines
	o.lines = nil
	return lines
}

type promptEvent struct {
	Type    string   `json:"type"`
	ID      int      `json:"id"`
	Op      string   `json:"op"`
	Context []string `json:"context"`
	Default bool     `json:"default"`
	launcher.Prompt
}

type pendingPrompt struct {
	event promptEvent
	reply chan promptReply
}

type promptReply struct {
	value string
	ok    bool
}

func (g *App) ask(p launcher.Prompt, def bool, op string, context []string) promptReply {
	pending := &pendingPrompt{reply: make(chan promptReply, 1)}
	g.promptMu.Lock()
	g.nextPrompt++
	pending.event = promptEvent{Type: "prompt", ID: g.nextPrompt, Op: op, Context: context, Default: def, Prompt: p}
	g.prompts[pending.event.ID] = pending
	g.promptMu.Unlock()
	g.emit(pending.event)
	g.host.post(g.host.showWindow)
	return <-pending.reply
}

func (g *App) resendPrompts() {
	g.promptMu.Lock()
	events := make([]promptEvent, 0, len(g.prompts))
	for _, p := range g.prompts {
		events = append(events, p.event)
	}
	g.promptMu.Unlock()
	sort.Slice(events, func(i, j int) bool { return events[i].ID < events[j].ID })
	for _, e := range events {
		g.emit(e)
	}
}

func (g *App) answer(id int, value string, ok bool) {
	g.promptMu.Lock()
	p := g.prompts[id]
	delete(g.prompts, id)
	g.promptMu.Unlock()
	if p != nil {
		p.reply <- promptReply{value: value, ok: ok}
	}
}
