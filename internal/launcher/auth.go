package launcher

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf16"
)

const (
	gameProjectID = "11321"
	oauthClientID = "gc.my.com"
	gameAgent     = "Downloader/19160"
	browserAgent  = "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.5414.120 Downloader/19160 MyComGameCenter/1916 Safari/537.36"
	bridgeAddr    = "127.0.0.1:51200"
)

var authCodePattern = regexp.MustCompile(`^[A-Za-z0-9_+/=-]{48}$`)

type oauthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int64  `json:"user_id"`
}

type gameTicket struct {
	XMLName     xml.Name `xml:"Login"`
	GameAccount string   `xml:"GameAccount,attr"`
	PersID      string   `xml:"PersId,attr"`
	Code        string   `xml:"Code,attr"`
}

type authBridge struct {
	state     string
	name      string
	codes     chan string
	server    *http.Server
	mu        sync.Mutex
	used      bool
	connected atomic.Bool
	asked     atomic.Bool
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func browserLoginURL(state string, fresh bool) string {
	callback := url.URL{Scheme: "https", Host: "api.vkplay.ru", Path: "/gamecenter/authcode/"}
	callback.RawQuery = "gc_id=0.0&state=" + url.QueryEscape(state)
	sdc := url.URL{Scheme: "https", Host: "auth-ac.vkplay.ru", Path: "/sdc"}
	sdc.RawQuery = url.Values{"from": {callback.String()}}.Encode()
	login := url.URL{Scheme: "https", Host: "account.vkplay.ru", Path: "/login/"}
	query := url.Values{"continue": {sdc.String()}}
	if fresh {
		query.Set("ignore_current_session", "1")
	}
	login.RawQuery = query.Encode()
	return login.String()
}

func tunnelPNG(message string) ([]byte, error) {
	units := utf16.Encode([]rune(message))
	data := make([]byte, len(units)*2)
	for i, unit := range units {
		data[2*i] = byte(unit)
		data[2*i+1] = byte(unit >> 8)
	}
	width := 1 + (len(data)+2)/3
	img := image.NewNRGBA(image.Rect(0, 0, width, 1))
	count := len(units)
	img.SetNRGBA(0, 0, color.NRGBA{R: byte(count), G: byte(count >> 8), B: byte(count >> 16), A: 255})
	for i, value := range data {
		pixel := 1 + i/3
		channel := i % 3
		at := img.PixOffset(pixel, 0) + channel
		img.Pix[at] = value
		img.Pix[img.PixOffset(pixel, 0)+3] = 255
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func decodeTunnelParam(encoded string) (string, error) {
	b, err := hex.DecodeString(encoded)
	if err != nil || len(b)%2 != 0 || len(b) > 16384 {
		return "", errors.New("invalid tunnel message")
	}
	units := make([]uint16, len(b)/2)
	for i := range units {
		units[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return string(utf16.Decode(units)), nil
}

func (b *authBridge) serveImage(w http.ResponseWriter, message string) {
	data, err := tunnelPNG(message)
	if err != nil {
		http.Error(w, "image encoding failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(data)
}

func jsonString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return ""
}

func (b *authBridge) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	switch r.URL.Path {
	case "/tun/init.png":
		b.connected.Store(true)
		b.serveImage(w, `{"Name":"`+b.name+`"}`)
	case "/tun/" + b.name + "/poll.png":
		if b.asked.CompareAndSwap(false, true) {
			b.serveImage(w, `{"Method":"GetAuthCode","State":"`+b.state+`"}`)
			return
		}
		b.serveImage(w, "")
	case "/tun/" + b.name + "/send.png":
		message, err := decodeTunnelParam(r.URL.Query().Get("param"))
		if err != nil {
			http.Error(w, "invalid message", http.StatusBadRequest)
			return
		}
		var call map[string]any
		if err := json.Unmarshal([]byte(message), &call); err != nil {
			b.serveImage(w, "")
			return
		}
		if jsonString(call["Method"]) != "SetAuthCode" {
			b.serveImage(w, "")
			return
		}
		code, state := jsonString(call["Code"]), jsonString(call["State"])
		codeOK := authCodePattern.MatchString(code)
		stateOK := subtle.ConstantTimeCompare([]byte(state), []byte(b.state)) == 1
		if !codeOK || !stateOK {
			fmt.Println("Sign-in response rejected; still waiting.")
		} else {
			b.mu.Lock()
			if !b.used {
				b.used = true
				b.codes <- code
			}
			b.mu.Unlock()
		}
		b.serveImage(w, "")
	default:
		http.NotFound(w, r)
	}
}

func startAuthBridge(state string) (*authBridge, func(), error) {
	name, err := randomHex(8)
	if err != nil {
		return nil, nil, err
	}
	listener, err := net.Listen("tcp", bridgeAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("local port %s is busy; close the program that uses it, such as another VK Play launcher: %w", bridgeAddr, err)
	}
	b := &authBridge{state: state, name: name, codes: make(chan string, 1)}
	b.server = &http.Server{Handler: http.HandlerFunc(b.handle), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = b.server.Serve(listener) }()
	return b, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = b.server.Shutdown(ctx)
	}, nil
}

type vkSignIn struct {
	URL      string
	FreshURL string
	Report   func(code, state string, err error)
}

var vkSignInStarted func(s vkSignIn) (done func())

type pageResult struct {
	code, state string
	err         error
}

var vkSignInMu sync.Mutex

var errSignInBusy = errors.New("another VK Play sign-in is in progress; finish or cancel it first")

func getBrowserCode(ctx context.Context) (string, error) {
	if !vkSignInMu.TryLock() {
		return "", errSignInBusy
	}
	defer vkSignInMu.Unlock()
	state, err := randomHex(16)
	if err != nil {
		return "", err
	}
	bridge, closeBridge, err := startAuthBridge(state)
	if err != nil {
		return "", err
	}
	defer closeBridge()
	loginURL, freshURL := browserLoginURL(state, false), browserLoginURL(state, true)
	results := make(chan pageResult, 4)
	report := func(code, state string, err error) {
		select {
		case results <- pageResult{code, state, err}:
		default:
		}
	}
	if err := openBrowser(loginURL); err != nil {
		return "", err
	}
	fmt.Println("VK Play sign-in opened in the browser. If you are signed in to VK Play there, it returns here by itself. Waiting...")
	if vkSignInStarted != nil {
		defer vkSignInStarted(vkSignIn{URL: loginURL, FreshURL: freshURL, Report: report})()
	}
	hint := time.NewTimer(20 * time.Second)
	defer hint.Stop()
	for {
		select {
		case code := <-bridge.codes:
			return code, nil
		case r := <-results:
			if r.err != nil {
				return "", r.err
			}
			if authCodePattern.MatchString(r.code) && subtle.ConstantTimeCompare([]byte(r.state), []byte(state)) == 1 {
				bridge.mu.Lock()
				bridge.used = true
				bridge.mu.Unlock()
				return r.code, nil
			}
			fmt.Println("Sign-in response rejected; still waiting.")
		case <-ctx.Done():
			return "", ctx.Err()
		case <-hint.C:
			if !bridge.connected.Load() {
				fmt.Printf("The browser has not reached the launcher yet. If it asks whether vkplay.ru may access apps on this device, allow it.\n"+
					"If it offers to open VK Games, cancel that: this browser does not let the page talk to programs on this computer.\n"+
					"Then open this link in Microsoft Edge or Google Chrome; the launcher keeps waiting:\n%s\n"+
					"To sign in with another account:\n%s\n", loginURL, freshURL)
			}
		}
	}
}

type httpStatusError struct {
	Host string
	Code int
}

func (e *httpStatusError) Error() string { return fmt.Sprintf("%s: HTTP %d", e.Host, e.Code) }

func authPOST(client *http.Client, endpoint, contentType string, body []byte, agent string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", agent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &httpStatusError{Host: req.URL.Host, Code: resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, errors.New("auth response too large")
	}
	return data, nil
}

func exchangeBrowserCode(client *http.Client, code string) (oauthTokens, error) {
	var tokens oauthTokens
	if !authCodePattern.MatchString(code) {
		return tokens, errors.New("invalid sign-in code")
	}
	body, _ := json.Marshal(map[string]string{"client_id": oauthClientID, "code": code})
	data, err := authPOST(client, "https://o2-ext-ac.vkplay.ru/api/v3/pub/oauth2/token", "application/json", body, browserAgent)
	if err != nil {
		return tokens, err
	}
	if err := json.Unmarshal(data, &tokens); err != nil {
		return tokens, errors.New("unexpected OAuth response")
	}
	if !authCodePattern.MatchString(tokens.RefreshToken) || tokens.UserID <= 0 {
		return oauthTokens{}, errors.New("OAuth returned no refresh_token")
	}
	return tokens, nil
}

func refreshSession(client *http.Client, refreshToken string) (string, string, error) {
	if !authCodePattern.MatchString(refreshToken) {
		return "", "", errors.New("invalid refresh_token")
	}
	form := url.Values{"client_id": {oauthClientID}, "grant_type": {"refresh_token"}, "refresh_token": {refreshToken}}
	data, err := authPOST(client, "https://o2-ac.vkplay.ru/token", "application/x-www-form-urlencoded", []byte(form.Encode()), browserAgent)
	if err != nil {
		return "", "", err
	}
	var response struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if json.Unmarshal(data, &response) != nil || !authCodePattern.MatchString(response.AccessToken) {
		return "", "", errors.New("unexpected O2 response")
	}
	if !authCodePattern.MatchString(response.RefreshToken) {
		response.RefreshToken = ""
	}
	return response.AccessToken, response.RefreshToken, nil
}

func requestGameTicket(client *http.Client, sessionKey string) (gameTicket, error) {
	var ticket gameTicket
	if !authCodePattern.MatchString(sessionKey) {
		return ticket, errors.New("invalid SessionKey")
	}
	body, err := xml.Marshal(struct {
		XMLName    xml.Name `xml:"Login"`
		SessionKey string   `xml:"SessionKey,attr"`
		ProjectID  string   `xml:"ProjectId,attr"`
	}{SessionKey: sessionKey, ProjectID: gameProjectID})
	if err != nil {
		return ticket, err
	}
	data, err := authPOST(client, "https://authdl.vkplay.ru/gem.php?hint=Login", "text/xml", body, gameAgent)
	if err != nil {
		return ticket, err
	}
	if xml.Unmarshal(data, &ticket) != nil || ticket.XMLName.Local != "Login" ||
		!authCodePattern.MatchString(ticket.Code) || ticket.GameAccount == "" {
		return gameTicket{}, errors.New("unexpected game token response")
	}
	return ticket, nil
}
