package launcher

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"net/http/httptest"
	"testing"
	"unicode/utf16"
)

func readTunnelPNG(t *testing.T, body []byte) string {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	nrgba := image.NewNRGBA(img.Bounds())
	draw.Draw(nrgba, nrgba.Bounds(), img, image.Point{}, draw.Src)
	first := nrgba.NRGBAAt(0, 0)
	count := int(first.R) | int(first.G)<<8 | int(first.B)<<16
	units := make([]uint16, count)
	for i := range units {
		lo, hi := 2*i, 2*i+1
		at := func(k int) byte { return nrgba.Pix[nrgba.PixOffset(1+k/3, 0)+k%3] }
		units[i] = uint16(at(lo)) | uint16(at(hi))<<8
	}
	return string(utf16.Decode(units))
}

func TestBridgeAsksPageForCodeOnce(t *testing.T) {
	b := &authBridge{state: "s1", name: "n1", codes: make(chan string, 1)}
	get := func(path string) string {
		w := httptest.NewRecorder()
		b.handle(w, httptest.NewRequest("GET", "http://127.0.0.1:51200"+path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: HTTP %d", path, w.Code)
		}
		return readTunnelPNG(t, w.Body.Bytes())
	}
	if got := get("/tun/init.png"); got != `{"Name":"n1"}` || !b.connected.Load() {
		t.Fatalf("init = %q", got)
	}
	if got := get("/tun/n1/poll.png"); got != `{"Method":"GetAuthCode","State":"s1"}` {
		t.Fatalf("first poll = %q", got)
	}
	if got := get("/tun/n1/poll.png"); got != "" {
		t.Fatalf("second poll = %q", got)
	}
}
