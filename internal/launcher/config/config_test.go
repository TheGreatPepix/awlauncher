package config

import (
	"testing"
)

func TestAccountsDefaultAndRemove(t *testing.T) {
	var c Config
	if _, ok := c.DefaultAccount(); ok {
		t.Fatal("an empty list must not yield a default account")
	}
	c.Upsert(Account{UserID: 1, Name: "main"})
	if a, ok := c.DefaultAccount(); !ok || a.UserID != 1 {
		t.Fatal("a single account must be the default")
	}
	c.Upsert(Account{UserID: 2})
	if _, ok := c.DefaultAccount(); ok {
		t.Fatal("two accounts without a last choice have no default")
	}
	c.LastUserID = 2
	c.Upsert(Account{UserID: 1})
	if a, _ := c.Find(1); a.Name != "main" {
		t.Fatalf("name lost: %+v", a)
	}
	c.Remove(2)
	if c.LastUserID != 0 || len(c.Accounts) != 1 {
		t.Fatalf("after remove: %+v", c)
	}
}

func TestConfigStoreChangesOnlyOneField(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	s := NewStore(Config{Accounts: []Account{{UserID: 1, Name: "old", Provider: ProviderFX, Email: "a@b.c"}}})
	held, _ := s.Find(1)
	if err := s.UpdateAccount(1, func(a *Account) { a.Name = "new" }); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateAccount(held.UserID, func(a *Account) { a.Branch = "supertest" }); err != nil {
		t.Fatal(err)
	}
	if a, _ := s.Find(1); a.Name != "new" || a.Branch != "supertest" {
		t.Fatalf("account = %+v", a)
	}
	if err := s.Update(func(c *Config) { c.Remove(1) }); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateAccount(1, func(a *Account) { a.Name = "late" }); err != nil {
		t.Fatal(err)
	}
	if len(s.Get().Accounts) != 0 {
		t.Fatal("a late change brought the account back")
	}
	saved, err := Load()
	if err != nil || len(saved.Accounts) != 0 {
		t.Fatalf("saved = %+v, %v", saved, err)
	}
}
