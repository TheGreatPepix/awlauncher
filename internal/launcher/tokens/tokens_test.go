package tokens

import (
	"errors"
	"testing"
)

func TestSessionRoundTrip(t *testing.T) {
	if _, err := Load(42); !errors.Is(err, ErrNeedLogin) {
		t.Fatalf("missing session: %v", err)
	}
	if err := Save(42, " refresh-1 \n"); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(42); err != nil || got != "refresh-1" {
		t.Fatalf("loaded %q, %v", got, err)
	}
	if err := Clear(42); err != nil {
		t.Fatal(err)
	}
	if err := Clear(42); err != nil {
		t.Fatalf("clearing twice: %v", err)
	}
	if _, err := Load(42); !errors.Is(err, ErrNeedLogin) {
		t.Fatalf("cleared session: %v", err)
	}
}
