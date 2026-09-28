package fxid

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"testing"
)

func fakeJWT(exp int64) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"HS256"}`)) + "." + enc([]byte(fmt.Sprintf(`{"exp":%d}`, exp))) + ".sig"
}

func TestLaunchArgs(t *testing.T) {
	for lang, name := range map[string]string{"ru": "russian", "en": "english"} {
		args := LaunchArgs(LaunchDefault, lang, "a@b", "JWT")
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
