package gui

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"os"
	"testing"
	"time"
)

type coreRun struct {
	in     *io.PipeWriter
	events chan map[string]any
	code   chan int
}

func startCore(t *testing.T) *coreRun {
	t.Helper()
	dataDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dataDir)
	t.Setenv("XDG_DATA_HOME", dataDir)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	r := &coreRun{in: inW, events: make(chan map[string]any, 100), code: make(chan int, 1)}
	go func() {
		r.code <- RunCore(inR, outW, "", false)
		outW.Close()
	}()
	go func() {
		lines := bufio.NewScanner(outR)
		lines.Buffer(nil, maxCommand)
		for lines.Scan() {
			var e map[string]any
			if err := json.Unmarshal(lines.Bytes(), &e); err != nil {
				t.Errorf("not a JSON event: %q", lines.Text())
				continue
			}
			r.events <- e
		}
		close(r.events)
	}()
	return r
}

func (r *coreRun) command(t *testing.T, c map[string]any) {
	t.Helper()
	data, _ := json.Marshal(c)
	if _, err := r.in.Write(append(data, '\n')); err != nil {
		t.Fatal(err)
	}
}

func (r *coreRun) next(t *testing.T, kind string) map[string]any {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case e, ok := <-r.events:
			if !ok {
				t.Fatalf("the core stopped before a %q event", kind)
			}
			if e["type"] == kind {
				return e
			}
		case <-timeout:
			t.Fatalf("no %q event", kind)
		}
	}
}

func (r *coreRun) exitCode(t *testing.T) int {
	t.Helper()
	select {
	case code := <-r.code:
		return code
	case <-time.After(5 * time.Second):
		t.Fatal("the core did not stop")
		return -1
	}
}

func TestCoreServesJSONLines(t *testing.T) {
	r := startCore(t)
	r.command(t, map[string]any{"cmd": "ready"})
	if tray := r.next(t, "tray"); tray["enabled"] != false {
		t.Fatalf("tray play is enabled without accounts: %v", tray)
	}
	state := r.next(t, "state")
	if accounts, ok := state["accounts"].([]any); !ok || len(accounts) != 0 {
		t.Fatalf("accounts = %v", state["accounts"])
	}
	if _, ok := state["prefs"]; !ok {
		t.Fatal("the first state has no prefs")
	}

	r.command(t, map[string]any{"cmd": "openDataFolder"})
	if open := r.next(t, "open"); open["target"] == "" {
		t.Fatal("open has no target")
	}

	r.command(t, map[string]any{"cmd": "applyUpdate"})
	r.next(t, "notice")

	r.command(t, map[string]any{"cmd": "requestExit"})
	r.next(t, "exit")
	if code := r.exitCode(t); code != 0 {
		t.Fatalf("exit code %d", code)
	}
}

func TestCoreStopsWhenWindowCloses(t *testing.T) {
	r := startCore(t)
	r.command(t, map[string]any{"cmd": "ready"})
	r.next(t, "state")
	r.in.Close()
	r.next(t, "exit")
	if code := r.exitCode(t); code != 0 {
		t.Fatalf("exit code %d", code)
	}
}

func TestCoreHidesProgressWhenWindowHidden(t *testing.T) {
	h := newCoreHost(io.Discard)
	g := newApp(nil, &logBuffer{}, false)
	in, out := io.Pipe()
	go h.loop()
	defer h.exit()
	go h.read(in, g)
	out.Write([]byte(`{"cmd":"visible","ok":false}` + "\n"))
	for range 100 {
		if !h.windowShown() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the window is still marked shown")
}
