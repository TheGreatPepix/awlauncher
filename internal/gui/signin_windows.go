package gui

import (
	"encoding/json"
	"log"
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

func (h *winHost) openSignIn(url string, report func(code, state string), closed func()) error {
	dark, surface, text := themeColors(loadPrefs().Theme)
	win, err := createHostWindow(signInClassName, "Sign in to VK Play — AWLauncher", h.win.hwnd, 560, 780, 420, 520, colorRef(surface), h.signInProc)
	if err != nil {
		return err
	}
	win.setTitleBar(dark, colorRef(surface), colorRef(text))
	setAppIcon(win)
	web, err := newWebView(win)
	if err != nil {
		win.destroy()
		return err
	}
	web.MessageCallback = h.signInMessage
	web.Init(signInScript)
	h.signWin, h.signWeb, h.signReport, h.signClosed = win, web, report, closed
	win.show()
	_ = web.Show()
	web.Resize()
	web.Navigate(url)
	return nil
}

func (h *winHost) showSignIn() {
	if h.signWin != nil {
		h.signWin.show()
	}
}

func (h *winHost) closeSignIn() {
	if w := h.signWin; w != nil {
		h.signWin, h.signWeb = nil, nil
		w.destroy()
	}
}

func (h *winHost) signInProc(hwnd uintptr, msg uint32, wp, _ uintptr) (uintptr, bool) {
	switch msg {
	case wmSize:
		if h.signWeb != nil {
			h.signWeb.Resize()
		}
	case wmMove:
		if h.signWeb != nil {
			_ = h.signWeb.NotifyParentWindowPositionChanged()
		}
	case wmActivate:
		if wp&0xFFFF != 0 && h.signWeb != nil {
			h.signWeb.Focus()
		}
	case wmDestroy:
		if h.signWin != nil && h.signWin.hwnd == hwnd {
			h.signWin, h.signWeb = nil, nil
		}
		if closed := h.signClosed; closed != nil {
			h.signClosed = nil
			closed()
		}
	}
	return 0, false
}

func (h *winHost) signInMessage(message string) {
	var m struct {
		Cmd, Code, State, Message string
	}
	if json.Unmarshal([]byte(message), &m) != nil {
		return
	}
	switch m.Cmd {
	case "code":
		if h.signReport != nil {
			h.signReport(m.Code, m.State)
		}
	case "error":
		log.Print("The sign-in page did not return the code: ", m.Message)
	}
}
