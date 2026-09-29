package fxid

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"testing"
)

func fakeJWT(exp int64) string {
	return jwtWith(fmt.Sprintf(`{"exp":%d}`, exp))
}

func jwtWith(payload string) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"HS256"}`)) + "." + enc([]byte(payload)) + ".sig"
}

func TestGameEnv(t *testing.T) {
	for payload, want := range map[string][]string{
		`{"external_id":108262461}`:   {"GC_PERS_ID=108262461"},
		`{"external_id":"108262461"}`: {"GC_PERS_ID=108262461"},
		`{"external_id":"..\\x"}`:     nil,
		`{"sub":"430"}`:               nil,
	} {
		if got := GameEnv(jwtWith(payload)); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got %q, want %q", payload, got, want)
		}
	}
	if GameEnv("not-a-jwt") != nil {
		t.Fatal("accepted a non-JWT")
	}
}

func TestLaunchArgs(t *testing.T) {
	for lang, name := range map[string]string{"ru": "russian", "pl": "polish"} {
		args := LaunchArgs(LaunchDefault, lang, name, "a@b", "JWT")
		if want := []string{"-pref_language", name, "--fxid-login-token=JWT"}; !reflect.DeepEqual(args, want) {
			t.Fatalf("args = %q, want %q", args, want)
		}
	}
}

func TestJWTExpiry(t *testing.T) {
	if exp, ok := JWTExpiry(fakeJWT(1790000000)); !ok || exp.Unix() != 1790000000 {
		t.Fatalf("exp = %v %v", exp, ok)
	}
	if _, ok := JWTExpiry("not-a-jwt"); ok {
		t.Fatal("accepted a non-JWT")
	}
}
