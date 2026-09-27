package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jchv/go-webview2/pkg/edge"
)

const signInClassName = "AWLauncherSignIn"

const signInScript = `(function () {
  if (location.hostname !== "api.vkplay.ru" || !location.pathname.startsWith("/gamecenter/authcode")) return;
  var page = new URLSearchParams(location.search), query = new URLSearchParams();
  ["gc_id", "remigrate", "state"].forEach(function (k) { var v = page.get(k); if (v) query.set(k, v); });
  function report(m) { window.chrome.webview.postMessage(JSON.stringify(m)); }
  fetch("https://account.vkplay.ru/account/get_gc_auth/?" + query, { credentials: "include" })
    .then(function (r) { return r.ok ? r.json() : Promise.reject("HTTP " + r.status); })
    .then(function (j) { report({ cmd: "code", code: j.code || "", state: page.get("state") || "" }); })
    .catch(function (e) { report({ cmd: "error", message: String(e) }); });
})();`

type signInState struct {
	vkSignIn
	win *hostWindow
	web *edge.Chromium
}

func (g *guiApp) vkSignInStarted(s vkSignIn) func() {
	g.post(func() {
		g.endSignIn()
		g.signIn = &signInState{vkSignIn: s}
		g.emitSignIn()
	})
	return func() { g.post(g.endSignIn) }
}

func (g *guiApp) emitSignIn() {
	if s := g.signIn; s != nil {
		g.emit(map[string]any{"type": "signin", "active": true, "window": s.win != nil})
	} else {
		g.emit(map[string]any{"type": "signin", "active": false})
	}
}

func (g *guiApp) signInCommand(cmd string) {
	s := g.signIn
	if s == nil {
		return
	}
	switch cmd {
	case "signinOpen":
		if err := openBrowser(s.URL); err != nil {
			fmt.Println("Error:", err)
		}
	case "signinFresh":
		if err := openBrowser(s.FreshURL); err != nil {
			fmt.Println("Error:", err)
		} else {
			fmt.Println("Sign in with the other account in the browser; the launcher keeps waiting.")
		}
	case "signinHere":
		if s.win != nil {
			s.win.show()
		} else if err := g.openSignInWindow(s); err != nil {
			fmt.Println("Cannot open the sign-in window:", err)
		}
	case "signinCancel":
		s.Report("", "", errQuit)
	}
}

func (g *guiApp) openSignInWindow(s *signInState) error {
	dark, surface, text := themeColors(loadPrefs().Theme)
	win, err := createHostWindow(signInClassName, "Sign in to VK Play — AWLauncher", g.win.hwnd, 560, 780, 420, 520, colorRef(surface), g.signInProc(s))
	if err != nil {
		return err
	}
	win.setTitleBar(dark, colorRef(surface), colorRef(text))
	setAppIcon(win)
	web := edge.NewChromium()
	web.MessageCallback = func(message string) { g.signInMessage(s, message) }
	if dir, err := launcherDir(); err == nil {
		web.DataPath = filepath.Join(dir, "WebView2")
	}
	if !web.Embed(win.hwnd) {
		win.destroy()
		return errors.New("cannot start Microsoft Edge WebView2")
	}
	clearLastError()
	if settings, err := web.GetSettings(); err == nil {
		_ = settings.PutAreDevToolsEnabled(os.Getenv("AWLAUNCHER_DEVTOOLS") == "1")
		_ = settings.PutIsStatusBarEnabled(false)
	}
	web.Init(signInScript)
	s.win, s.web = win, web
	win.show()
	_ = web.Show()
	web.Resize()
	web.Navigate(s.FreshURL)
	fmt.Println("VK Play sign-in opened in the launcher window.")
	g.emitSignIn()
	return nil
}

func (g *guiApp) signInProc(s *signInState) func(uintptr, uint32, uintptr, uintptr) (uintptr, bool) {
	return func(hwnd uintptr, msg uint32, wp, _ uintptr) (uintptr, bool) {
		switch msg {
		case wmSize:
			if s.web != nil {
				s.web.Resize()
			}
		case wmMove:
			if s.web != nil {
				_ = s.web.NotifyParentWindowPositionChanged()
			}
		case wmActivate:
			if wp&0xFFFF != 0 && s.web != nil {
				s.web.Focus()
			}
		case wmDestroy:
			if s.win != nil && s.win.hwnd == hwnd {
				s.win, s.web = nil, nil
				if g.signIn == s {
					g.emitSignIn()
				}
			}
		}
		return 0, false
	}
}

func (g *guiApp) signInMessage(s *signInState, message string) {
	var m struct {
		Cmd, Code, State, Message string
	}
	if json.Unmarshal([]byte(message), &m) != nil {
		return
	}
	switch m.Cmd {
	case "code":
		s.Report(m.Code, m.State, nil)
	case "error":
		fmt.Println("The sign-in page did not return the code:", m.Message)
	}
}

func (g *guiApp) endSignIn() {
	s := g.signIn
	if s == nil {
		return
	}
	g.signIn = nil
	if s.win != nil {
		w := s.win
		s.win, s.web = nil, nil
		w.destroy()
	}
	g.emitSignIn()
}
