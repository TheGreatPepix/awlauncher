package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TheGreatPepix/awlauncher/internal/region"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var errNeedLogin = errors.New("account session is no longer valid, sign in again")

func authClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

func loginAccount(client *http.Client, cfg *configStore, name string, timeout time.Duration) (account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	code, err := getBrowserCode(ctx)
	if errors.Is(err, context.DeadlineExceeded) {
		return account{}, fmt.Errorf("sign-in not completed within %s", timeout)
	}
	if err != nil {
		return account{}, err
	}
	tokens, err := exchangeBrowserCode(client, code)
	if err != nil {
		return account{}, err
	}
	if err := saveRefreshToken(tokens.UserID, tokens.RefreshToken); err != nil {
		return account{}, fmt.Errorf("save session: %w", err)
	}
	acc, err := cfg.signedIn(account{UserID: tokens.UserID, Name: name})
	fmt.Printf("Signed in: %s\n", acc.label())
	if bringLauncherForward != nil {
		bringLauncherForward()
	}
	return acc, err
}

var bringLauncherForward func()

const vkSignInTimeout = 15 * time.Minute

func sessionFor(client *http.Client, userID int64) (string, error) {
	token, err := loadRefreshToken(userID)
	if errors.Is(err, os.ErrNotExist) {
		return "", errNeedLogin
	}
	if err != nil {
		return "", fmt.Errorf("read session: %w", err)
	}
	key, rotated, err := refreshSession(client, token)
	if err != nil {
		var status *httpStatusError
		if errors.As(err, &status) && status.Code >= 400 && status.Code < 500 {
			_ = clearRefreshToken(userID)
			return "", errNeedLogin
		}
		return "", fmt.Errorf("refresh session: %w", err)
	}
	if rotated != "" && rotated != token {
		if err := saveRefreshToken(userID, rotated); err != nil {
			return "", fmt.Errorf("save session: %w", err)
		}
	}
	return key, nil
}

type gameInstall struct {
	Root   string
	Build  int
	Launch launchConfig
	Exe    string
}

func openGame(dir string) (gameInstall, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return gameInstall{}, err
	}
	build, last, err := currentBuild(root)
	if err != nil {
		return gameInstall{}, fmt.Errorf("read game build: %w", err)
	}
	lc, err := readLaunchConfig(last)
	if err != nil {
		return gameInstall{}, err
	}
	exe, err := safeGamePath(root, lc.Exe)
	if err != nil {
		return gameInstall{}, err
	}
	return gameInstall{Root: root, Build: build, Launch: lc, Exe: exe}, nil
}

func startGame(client *http.Client, g gameInstall, acc account) error {
	if name, err := runningProcess("ArmoredWarfare.exe"); err != nil {
		return err
	} else if name != "" {
		return errors.New("the game is already running")
	}
	key, err := sessionFor(client, acc.UserID)
	if err != nil {
		return err
	}
	if err := region.Ensure(g.Root, region.VK); err != nil {
		return fmt.Errorf("switch to the VK Play servers: %w", err)
	}
	ticket, err := requestGameTicket(client, key)
	if err != nil {
		return err
	}
	pid, err := startGameProcess(g.Exe, g.Launch.args(ticket), g.Root)
	if err != nil {
		return fmt.Errorf("start the game: %w", err)
	}
	fmt.Printf("Game started: %s, PID %d\n", acc.label(), pid)
	return nil
}

func dropSession(userID int64) error {
	if token, err := loadRefreshToken(userID); err == nil {
		body, _ := json.Marshal(map[string]string{"client_id": oauthClientID, "refresh_token": token})
		if _, err := authPOST(authClient(), "https://o2-ext-ac.vkplay.ru/api/v3/pub/oauth2/drop", "application/json", body, browserAgent); err != nil {
			fmt.Fprintln(os.Stderr, "Warning: the server did not confirm session revocation:", err)
		}
	}
	return clearRefreshToken(userID)
}
