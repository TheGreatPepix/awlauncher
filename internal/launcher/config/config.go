package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
)

type Account struct {
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	Provider string `json:"provider,omitempty"`
	Email    string `json:"email,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Language string `json:"language,omitempty"`
}

func (a Account) IsFX() bool { return a.Provider == ProviderFX }
func (a Account) Service() string {
	if a.IsFX() && a.Branch != "" {
		return "FX ID, " + a.Branch
	}
	if a.IsFX() {
		return "FX ID"
	}
	return "VK Play"
}
func (a Account) Login() string {
	if a.IsFX() {
		return a.Email
	}
	return strconv.FormatInt(a.UserID, 10)
}
func (a Account) DisplayName() string {
	if a.Name != "" {
		return a.Name
	}
	return a.Login()
}
func (a Account) Label() string {
	id := strconv.FormatInt(a.UserID, 10)
	if a.IsFX() {
		id = a.Email
	}
	if a.Name != "" && a.Name != id {
		return fmt.Sprintf("%s (%s)", a.Name, id)
	}
	return id
}

type Config struct {
	Game        string            `json:"game"`
	FXGame      string            `json:"fx_game,omitempty"`
	AllowMods   bool              `json:"allow_mods,omitempty"`
	NoBackups   bool              `json:"no_patch_backups,omitempty"`
	KeepOpen    bool              `json:"keep_open,omitempty"`
	BranchGames map[string]string `json:"branch_games,omitempty"`
	Accounts    []Account         `json:"accounts"`
	LastUserID  int64             `json:"last_user_id"`
}

func Load() (Config, error) {
	var cfg Config
	dir, err := platform.DataDir()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("config.json: %w", err)
	}
	return cfg, nil
}
func Save(cfg Config) error {
	dir, err := platform.DataDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, "config.json.tmp")
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "config.json"))
}

type Store struct {
	mu  sync.Mutex
	cfg Config
}

func NewStore(cfg Config) *Store { return &Store{cfg: cfg} }
func (s *Store) Get() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.cfg
	c.Accounts = append([]Account(nil), s.cfg.Accounts...)
	c.BranchGames = make(map[string]string, len(s.cfg.BranchGames))
	for branch, dir := range s.cfg.BranchGames {
		c.BranchGames[branch] = dir
	}
	return c
}
func (s *Store) Find(userID int64) (Account, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Find(userID)
}
func (s *Store) Update(change func(c *Config)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	change(&s.cfg)
	return Save(s.cfg)
}
func (s *Store) UpdateAccount(userID int64, change func(a *Account)) error {
	return s.Update(func(c *Config) {
		for i := range c.Accounts {
			if c.Accounts[i].UserID == userID {
				change(&c.Accounts[i])
				return
			}
		}
	})
}
func (s *Store) SignedIn(acc Account) (Account, error) {
	var saved Account
	err := s.Update(func(c *Config) {
		c.Upsert(acc)
		saved, _ = c.Find(acc.UserID)
		c.LastUserID = acc.UserID
	})
	return saved, err
}
func (c *Config) Find(userID int64) (Account, bool) {
	for _, a := range c.Accounts {
		if a.UserID == userID {
			return a, true
		}
	}
	return Account{}, false
}
func (c *Config) Upsert(a Account) {
	for i := range c.Accounts {
		if c.Accounts[i].UserID == a.UserID {
			if a.Name == "" {
				a.Name = c.Accounts[i].Name
			}
			if a.Language == "" {
				a.Language = c.Accounts[i].Language
			}
			c.Accounts[i] = a
			return
		}
	}
	c.Accounts = append(c.Accounts, a)
}
func (c *Config) Remove(userID int64) {
	kept := c.Accounts[:0]
	for _, a := range c.Accounts {
		if a.UserID != userID {
			kept = append(kept, a)
		}
	}
	c.Accounts = kept
	if c.LastUserID == userID {
		c.LastUserID = 0
	}
}
func (c *Config) DefaultAccount() (Account, bool) {
	if a, ok := c.Find(c.LastUserID); ok {
		return a, true
	}
	if len(c.Accounts) == 1 {
		return c.Accounts[0], true
	}
	return Account{}, false
}

func (c Config) BranchDir(branch string) string {
	if dir := c.BranchGames[strings.ToLower(branch)]; dir != "" {
		return dir
	}
	root := c.FXGame
	if root == "" {
		root = DefaultFXDir()
	}
	return gamefiles.BranchDir(root, branch)
}

func DefaultFXDir() string { return platform.DefaultGameDir() + " FX ID" }

const ProviderFX = "fxid"
