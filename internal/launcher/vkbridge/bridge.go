package vkbridge

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
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

const bridgeAddr = "127.0.0.1:51200"

var authCodePattern = regexp.MustCompile(`^[A-Za-z0-9_+/=-]{48}$`)

func ValidCode(code string) bool { return authCodePattern.MatchString(code) }

type Bridge struct {
	state     string
	name      string
	codes     chan string
	server    *http.Server
	mu        sync.Mutex
	used      bool
	connected atomic.Bool
	asked     atomic.Bool
}

func RandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func LoginURL(state string, fresh bool) string {
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

func DecodeTunnelParam(encoded string) (string, error) {
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

func (b *Bridge) serveImage(w http.ResponseWriter, message string) {
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

func (b *Bridge) handle(w http.ResponseWriter, r *http.Request) {
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
		message, err := DecodeTunnelParam(r.URL.Query().Get("param"))
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
		codeOK := ValidCode(code)
		stateOK := subtle.ConstantTimeCompare([]byte(state), []byte(b.state)) == 1
		if !codeOK || !stateOK {
			log.Println("Sign-in response rejected; still waiting.")
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

func Start(state string) (*Bridge, func(), error) {
	name, err := RandomHex(8)
	if err != nil {
		return nil, nil, err
	}
	listener, err := net.Listen("tcp", bridgeAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("local port %s is busy; close the program that uses it, such as another VK Play launcher: %w", bridgeAddr, err)
	}
	b := &Bridge{state: state, name: name, codes: make(chan string, 1)}
	b.server = &http.Server{Handler: http.HandlerFunc(b.handle), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = b.server.Serve(listener) }()
	return b, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = b.server.Shutdown(ctx)
	}, nil
}

func (b *Bridge) Codes() <-chan string { return b.codes }

func (b *Bridge) Connected() bool { return b.connected.Load() }

func (b *Bridge) MarkUsed() {
	b.mu.Lock()
	b.used = true
	b.mu.Unlock()
}
