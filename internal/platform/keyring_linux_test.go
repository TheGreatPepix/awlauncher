package platform

import (
	"bytes"
	"errors"
	"testing"
)

func withKey(t *testing.T, key []byte, err error) {
	t.Helper()
	old := tokenKey
	tokenKey = func(bool) ([]byte, error) { return key, err }
	t.Cleanup(func() { tokenKey = old })
}

func TestSealedSessions(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	withKey(t, key, nil)
	sealed, err := Protect([]byte("refresh-token"))
	if err != nil || !bytes.HasPrefix(sealed, sealedMagic) || bytes.Contains(sealed, []byte("refresh")) {
		t.Fatalf("Protect = %q, %v", sealed, err)
	}
	if clear, err := Unprotect(sealed); err != nil || string(clear) != "refresh-token" {
		t.Fatalf("Unprotect = %q, %v", clear, err)
	}
	if clear, err := Unprotect([]byte("plain-token")); err != nil || string(clear) != "plain-token" {
		t.Fatalf("a plain session should load as it is: %q, %v", clear, err)
	}

	withKey(t, bytes.Repeat([]byte{8}, 32), nil)
	if _, err := Unprotect(sealed); !errors.Is(err, ErrKeyUnavailable) {
		t.Fatalf("another key: %v", err)
	}
	withKey(t, nil, ErrKeyUnavailable)
	if _, err := Unprotect(sealed); !errors.Is(err, ErrKeyUnavailable) {
		t.Fatalf("a locked keyring: %v", err)
	}
	if plain, err := Protect([]byte("refresh-token")); err != nil || string(plain) != "refresh-token" {
		t.Fatalf("without a keyring sessions stay plain: %q, %v", plain, err)
	}
}
