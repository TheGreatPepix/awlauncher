package launcher

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

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

func (g *guiApp) opSession(op operation) *session {
	out := &opOutput{}
	return g.session.with(prompter{
		ask: func(question string) string {
			r := g.prompt("ask", question, false, op.Title, out.take())
			if !r.ok {
				return ""
			}
			return r.value
		},
		confirm: func(question string, def bool) bool {
			r := g.prompt("confirm", question, def, op.Title, out.take())
			return r.ok && r.value == "yes"
		},
		note: out.note,
		notify: func(text string) {
			g.emit(map[string]any{"type": "notice", "message": text, "kind": "ok"})
		},
	})
}

func (g *guiApp) prompt(kind, question string, def bool, op string, context []string) promptReply {
	p := &pendingPrompt{kind: kind, question: strings.TrimSpace(question), def: def, op: op, context: context, reply: make(chan promptReply, 1)}
	g.promptMu.Lock()
	g.nextPrompt++
	id := g.nextPrompt
	g.prompts[id] = p
	g.promptMu.Unlock()
	fmt.Fprintf(os.Stdout, "%s%d\n", promptMarker, id)
	return <-p.reply
}

func (g *guiApp) openPrompt(id int) {
	g.promptMu.Lock()
	p := g.prompts[id]
	g.promptMu.Unlock()
	if p == nil {
		return
	}
	event := map[string]any{
		"type": "prompt", "id": id, "kind": p.kind, "question": p.question, "default": p.def, "op": p.op, "context": p.context,
	}
	if p.kind == "ask" && strings.Contains(strings.ToLower(p.question), "folder") {
		event["folder"] = true
		cfg := g.store.Get()
		suggest := suggestGameFolder(cfg.Game)
		if p.op == "FX ID game folder" || strings.Contains(p.question, "FX ID game folder") {
			suggest = cfg.FXGame
			if suggest == "" {
				suggest = platform.DefaultGameDir() + " FX ID"
			}
		} else if strings.HasPrefix(p.op, "FX ID ") {
			branch := strings.TrimPrefix(p.op, "FX ID ")
			suggest = cfg.BranchDir(branch)
			event["targetBranch"] = branch
		}
		event["suggest"] = suggest
	}
	g.emit(event)
	g.post(g.showWindow)
}

type folderInfo struct {
	Type    string `json:"type"`
	Prompt  int    `json:"prompt"`
	Path    string `json:"path"`
	Valid   bool   `json:"valid"`
	Free    int64  `json:"free"`
	Drive   string `json:"drive"`
	Install bool   `json:"install"`
	Branch  string `json:"branch"`
	Used    bool   `json:"used"`
}

func describeFolder(prompt int, dir string) folderInfo {
	info := folderInfo{Type: "folderInfo", Prompt: prompt, Path: dir}
	dir = strings.Trim(strings.TrimSpace(dir), `"'`)
	if dir == "" || !filepath.IsAbs(dir) {
		return info
	}
	info.Drive = filepath.VolumeName(dir)
	if _, err := os.Stat(info.Drive + `\`); err != nil {
		return info
	}
	info.Valid = true
	info.Free, _ = platform.DiskFree(dir)
	if state, ok := gamefiles.ReadBranchState(dir); ok {
		if state.Branch == gamefiles.DefaultBranch {
			info.Install = true
		} else {
			info.Branch = state.Branch
		}
	}
	info.Install = info.Install || gamefiles.IsVKInstall(dir)
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		info.Used = true
	}
	return info
}

func suggestGameFolder(saved string) string {
	if saved != "" {
		return saved
	}
	return platform.DefaultGameDir()
}

func (g *guiApp) answer(id int, value string, ok bool) {
	g.promptMu.Lock()
	p := g.prompts[id]
	delete(g.prompts, id)
	g.promptMu.Unlock()
	if p != nil {
		p.reply <- promptReply{value: value, ok: ok}
	}
}

func (g *guiApp) captureOutput() error {
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	os.Stdout, os.Stderr = writer, writer
	go func() {
		lines := bufio.NewReader(reader)
		for {
			line, err := lines.ReadString('\n')
			if line != "" {
				g.output(line)
			}
			if err != nil {
				return
			}
		}
	}()
	return nil
}

func (g *guiApp) output(line string) {
	if rest, ok := strings.CutPrefix(line, promptMarker); ok {
		id, _ := strconv.Atoi(strings.TrimSpace(rest))
		g.openPrompt(id)
		return
	}
	line = strings.TrimRight(line, "\r\n") + "\n"
	g.outMu.Lock()
	g.logText = append(g.logText, line...)
	if len(g.logText) > logLimit {
		cut := len(g.logText) - logLimit*3/4
		if i := strings.IndexByte(string(g.logText[cut:]), '\n'); i >= 0 {
			cut += i + 1
		}
		g.logText = append([]byte(nil), g.logText[cut:]...)
	}
	g.outMu.Unlock()
	g.emit(map[string]any{"type": "log", "text": line})
}

func (g *guiApp) watchProgress() {
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	wasActive := false
	for range tick.C {
		s := progress.Default.Snapshot()
		if s.Active || s.Paused || wasActive {
			g.emit(struct {
				Type string `json:"type"`
				progress.Snapshot
			}{"progress", s})
		}
		wasActive = s.Active || s.Paused
	}
}
