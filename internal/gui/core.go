package gui

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log"
	"sync"
	"sync/atomic"

	"github.com/TheGreatPepix/awlauncher/internal/gui/startup"
	"github.com/TheGreatPepix/awlauncher/internal/gui/ui"
	"github.com/TheGreatPepix/awlauncher/internal/gui/update"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
)

const maxCommand = 16 << 20

// coreHost serves the launcher to a separate window process: commands come in
// and events go out as JSON lines, and the window draws itself, the tray and
// the dialogs of the system.
type coreHost struct {
	outMu sync.Mutex
	out   io.Writer

	queueMu sync.Mutex
	queue   []func()
	wake    chan struct{}
	done    chan struct{}
	exiting sync.Once
	shown   atomic.Bool
}

func newCoreHost(out io.Writer) *coreHost {
	h := &coreHost{out: out, wake: make(chan struct{}, 1), done: make(chan struct{})}
	h.shown.Store(true)
	return h
}

func (h *coreHost) loop() {
	for {
		select {
		case <-h.wake:
		case <-h.done:
			return
		}
		h.queueMu.Lock()
		q := h.queue
		h.queue = nil
		h.queueMu.Unlock()
		for _, f := range q {
			f()
		}
	}
}

func (h *coreHost) post(f func()) {
	h.queueMu.Lock()
	h.queue = append(h.queue, f)
	h.queueMu.Unlock()
	select {
	case h.wake <- struct{}{}:
	default:
	}
}

func (h *coreHost) send(event []byte) {
	h.outMu.Lock()
	defer h.outMu.Unlock()
	_, _ = h.out.Write(append(append([]byte(nil), event...), '\n'))
}

func (h *coreHost) sendEvent(event any) {
	if data, err := json.Marshal(event); err == nil {
		h.send(data)
	}
}

func (h *coreHost) showWindow()       { h.sendEvent(map[string]any{"type": "window", "show": true}) }
func (h *coreHost) hideWindow()       { h.sendEvent(map[string]any{"type": "window", "show": false}) }
func (h *coreHost) windowShown() bool { return h.shown.Load() }

func (h *coreHost) exit() {
	h.exiting.Do(func() {
		h.sendEvent(map[string]any{"type": "exit"})
		close(h.done)
	})
}

func (h *coreHost) open(target string) { h.sendEvent(map[string]any{"type": "open", "target": target}) }

func (h *coreHost) setTrayPlay(text string, enabled bool) {
	h.sendEvent(map[string]any{"type": "tray", "play": text, "enabled": enabled})
}

var errWindowOnly = errors.New("the window does this itself")

func (h *coreHost) setTitleBar(bool, [3]uint8, [3]uint8) {}
func (h *coreHost) pickFolder(string) (string, error)    { return "", errWindowOnly }
func (h *coreHost) copyText(string) error                { return errWindowOnly }
func (h *coreHost) relabelTray()                         {}
func (h *coreHost) showSignIn()                          {}
func (h *coreHost) closeSignIn()                         {}
func (h *coreHost) openSignIn(string, func(string, string), func()) error {
	return errors.New("sign-in inside AWLauncher is not available; use the browser")
}

// RunCore runs the launcher without a window, for the window process that
// started it. app is that process's executable, which starts with Windows;
// updated says the installer of a newer version started it.
func RunCore(in io.Reader, out io.Writer, app string, updated bool) int {
	ui.SetLanguage(loadPrefs().Lang)
	if app != "" {
		startup.SetTarget(app)
	}
	h := newCoreHost(out)
	logs := &logBuffer{}
	defer startLog(logs)()
	cfg, err := config.Load()
	if err != nil {
		h.sendEvent(map[string]any{"type": "fatal", "message": err.Error()})
		return 1
	}
	g := newApp(config.NewStore(cfg), logs, updated)
	g.host, g.core, g.appExe = h, true, app
	if app != "" {
		go update.RemoveLeftovers(app)
	}
	logs.follow(g.emitLog)
	startup.Refresh()
	go g.watchProgress()
	go g.watchGame()
	go h.read(in, g)
	h.loop()
	return 0
}

func (h *coreHost) read(in io.Reader, g *App) {
	lines := bufio.NewScanner(in)
	lines.Buffer(make([]byte, 64<<10), maxCommand)
	for lines.Scan() {
		line := lines.Text()
		var c struct {
			Cmd string `json:"cmd"`
			OK  bool   `json:"ok"`
		}
		if json.Unmarshal([]byte(line), &c) != nil {
			continue
		}
		if c.Cmd == "visible" {
			h.shown.Store(c.OK)
			continue
		}
		h.post(func() { g.onMessage(line) })
	}
	if err := lines.Err(); err != nil {
		log.Print("The window connection is broken: ", err)
	}
	gamefiles.WaitInstall()
	h.post(h.exit)
}
