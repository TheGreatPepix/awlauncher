package tokens

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/platform"
)

var ErrNeedLogin = errors.New("account session is no longer valid, sign in again")

func file(userID int64) (string, error) {
	dir, err := platform.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "accounts", strconv.FormatInt(userID, 10)+".bin"), nil
}

func Save(userID int64, token string) error {
	path, err := file(userID)
	if err != nil {
		return err
	}
	protected, err := platform.Protect([]byte(token))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, protected, 0600)
}

func Load(userID int64) (string, error) {
	path, err := file(userID)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNeedLogin
	}
	if err != nil {
		return "", err
	}
	clear, err := platform.Unprotect(data)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(clear)), nil
}

func Clear(userID int64) error {
	path, err := file(userID)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
