package launcher

import (
	"context"
	"crypto/subtle"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/vkplay/bridge"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
)

type pageResult struct {
	code, state string
	err         error
}

var vkSignInMu sync.Mutex

var errSignInBusy = errors.New("another VK Play sign-in is in progress; finish or cancel it first")

func (s *Session) browserCode(ctx context.Context) (string, error) {
	if !vkSignInMu.TryLock() {
		return "", errSignInBusy
	}
	defer vkSignInMu.Unlock()
	state, err := bridge.RandomHex(16)
	if err != nil {
		return "", err
	}
	srv, closeBridge, err := bridge.Start(state)
	if err != nil {
		return "", err
	}
	defer closeBridge()
	loginURL, freshURL := bridge.LoginURL(state, false), bridge.LoginURL(state, true)
	results := make(chan pageResult, 4)
	report := func(code, state string, err error) {
		select {
		case results <- pageResult{code, state, err}:
		default:
		}
	}
	if err := platform.OpenBrowser(loginURL); err != nil {
		return "", err
	}
	log.Print("VK Play sign-in opened in the browser. If you are signed in to VK Play there, it returns here by itself. Waiting...")
	defer s.ui.VKSignIn(VKSignIn{URL: loginURL, FreshURL: freshURL, Report: report})()
	hint := time.NewTimer(20 * time.Second)
	defer hint.Stop()
	for {
		select {
		case code := <-srv.Codes():
			return code, nil
		case r := <-results:
			if r.err != nil {
				return "", r.err
			}
			if bridge.ValidCode(r.code) && subtle.ConstantTimeCompare([]byte(r.state), []byte(state)) == 1 {
				srv.MarkUsed()
				return r.code, nil
			}
			log.Print("Sign-in response rejected; still waiting.")
		case <-ctx.Done():
			return "", ctx.Err()
		case <-hint.C:
			if !srv.Connected() {
				log.Printf("The browser has not reached the launcher yet. If it asks whether vkplay.ru may access apps on this device, allow it.\n"+
					"If it offers to open VK Games, cancel that: this browser does not let the page talk to programs on this computer.\n"+
					"Then open this link in your browser; the launcher keeps waiting:\n%s\n"+
					"To sign in with another account:\n%s", loginURL, freshURL)
			}
		}
	}
}
