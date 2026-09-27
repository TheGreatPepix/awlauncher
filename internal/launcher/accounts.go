package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type account struct {
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	Provider string `json:"provider,omitempty"`
	Email    string `json:"email,omitempty"`
	Branch   string `json:"branch,omitempty"`
}

func (a account) isFX() bool { return a.Provider == providerFX }

func (a account) service() string {
	if a.isFX() && a.Branch != "" {
		return "FX ID, " + a.Branch
	}
	if a.isFX() {
		return "FX ID"
	}
	return "VK Play"
}

func (a account) login() string {
	if a.isFX() {
		return a.Email
	}
	return strconv.FormatInt(a.UserID, 10)
}

func (a account) displayName() string {
	if a.Name != "" {
		return a.Name
	}
	return a.login()
}

func (a account) label() string {
	id := strconv.FormatInt(a.UserID, 10)
	if a.isFX() {
		id = a.Email
	}
	if a.Name != "" && a.Name != id {
		return fmt.Sprintf("%s (%s)", a.Name, id)
	}
	return id
}

type launcherConfig struct {
	Game       string    `json:"game"`
	FXGame     string    `json:"fx_game,omitempty"`
	Accounts   []account `json:"accounts"`
	LastUserID int64     `json:"last_user_id"`
}

func loadConfig() (launcherConfig, error) {
	var cfg launcherConfig
	dir, err := launcherDir()
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
	if cfg.Game == "" && cfg.FXGame != "" {
		cfg.Game, cfg.FXGame = cfg.FXGame, ""
	}
	return cfg, nil
}

func saveConfig(cfg launcherConfig) error {
	dir, err := launcherDir()
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

type configStore struct {
	mu  sync.Mutex
	cfg launcherConfig
}

func newConfigStore(cfg launcherConfig) *configStore { return &configStore{cfg: cfg} }

func (s *configStore) get() launcherConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.cfg
	c.Accounts = append([]account(nil), s.cfg.Accounts...)
	return c
}

func (s *configStore) find(userID int64) (account, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.find(userID)
}

func (s *configStore) update(change func(c *launcherConfig)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	change(&s.cfg)
	return saveConfig(s.cfg)
}

func (s *configStore) updateAccount(userID int64, change func(a *account)) error {
	return s.update(func(c *launcherConfig) {
		for i := range c.Accounts {
			if c.Accounts[i].UserID == userID {
				change(&c.Accounts[i])
				return
			}
		}
	})
}

func (s *configStore) signedIn(acc account) (account, error) {
	var saved account
	err := s.update(func(c *launcherConfig) {
		c.upsert(acc)
		saved, _ = c.find(acc.UserID)
		c.LastUserID = acc.UserID
	})
	return saved, err
}

func (c *launcherConfig) find(userID int64) (account, bool) {
	for _, a := range c.Accounts {
		if a.UserID == userID {
			return a, true
		}
	}
	return account{}, false
}

func (c *launcherConfig) upsert(a account) {
	for i := range c.Accounts {
		if c.Accounts[i].UserID == a.UserID {
			if a.Name == "" {
				a.Name = c.Accounts[i].Name
			}
			c.Accounts[i] = a
			return
		}
	}
	c.Accounts = append(c.Accounts, a)
}

func (c *launcherConfig) remove(userID int64) {
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

func (c *launcherConfig) defaultAccount() (account, bool) {
	if a, ok := c.find(c.LastUserID); ok {
		return a, true
	}
	if len(c.Accounts) == 1 {
		return c.Accounts[0], true
	}
	return account{}, false
}

func tokenFile(userID int64) (string, error) {
	dir, err := launcherDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "accounts", strconv.FormatInt(userID, 10)+".bin"), nil
}

func saveRefreshToken(userID int64, token string) error {
	path, err := tokenFile(userID)
	if err != nil {
		return err
	}
	protected, err := protectForUser([]byte(token))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, protected, 0600)
}

func loadRefreshToken(userID int64) (string, error) {
	path, err := tokenFile(userID)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	clear, err := unprotectForUser(data)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(clear)), nil
}

func clearRefreshToken(userID int64) error {
	path, err := tokenFile(userID)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func isGameDir(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "-gup-", "last.xml"))
	return err == nil
}

var gameDirNames = []string{
	"Games/Armored Warfare", "GamesMailRu/Armored Warfare", "VK Play/Armored Warfare",
	"WishlistGames/Armored Warfare", "Wishlist Games/Armored Warfare",
	"Armored Warfare",
}

func detectGameDir() string {
	for _, root := range fixedDrives() {
		for _, sub := range gameDirNames {
			if dir := filepath.Join(root, filepath.FromSlash(sub)); isGameDir(dir) {
				return dir
			}
		}
	}
	return ""
}
