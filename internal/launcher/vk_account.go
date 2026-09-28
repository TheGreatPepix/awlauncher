package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/vkauth"
)

var errNeedLogin = errors.New("account session is no longer valid, sign in again")

func authClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }
func loginAccount(client *http.Client, cfg *config.Store, name string, timeout time.Duration) (config.Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	code, err := getBrowserCode(ctx)
	if errors.Is(err, context.DeadlineExceeded) {
		return config.Account{}, fmt.Errorf("sign-in not completed within %s", timeout)
	}
	if err != nil {
		return config.Account{}, err
	}
	tokens, err := vkauth.ExchangeBrowserCode(client, code)
	if err != nil {
		return config.Account{}, err
	}
	if err := config.SaveRefreshToken(tokens.UserID, tokens.RefreshToken); err != nil {
		return config.Account{}, fmt.Errorf("save session: %w", err)
	}
	acc, err := cfg.SignedIn(config.Account{UserID: tokens.UserID, Name: name})
	fmt.Printf("Signed in: %s\n", acc.Label())
	if bringLauncherForward != nil {
		bringLauncherForward()
	}
	return acc, err
}

var bringLauncherForward func()

const vkSignInTimeout = 15 * time.Minute

func sessionFor(client *http.Client, userID int64) (string, error) {
	token, err := config.LoadRefreshToken(userID)
	if errors.Is(err, os.ErrNotExist) {
		return "", errNeedLogin
	}
	if err != nil {
		return "", fmt.Errorf("read session: %w", err)
	}
	key, rotated, err := vkauth.RefreshSession(client, token)
	if err != nil {
		var status *vkauth.HTTPStatusError
		if errors.As(err, &status) && status.Code >= 400 && status.Code < 500 {
			_ = config.ClearRefreshToken(userID)
			return "", errNeedLogin
		}
		return "", fmt.Errorf("refresh session: %w", err)
	}
	if rotated != "" && rotated != token {
		if err := config.SaveRefreshToken(userID, rotated); err != nil {
			return "", fmt.Errorf("save session: %w", err)
		}
	}
	return key, nil
}
func startGame(client *http.Client, g gamefiles.Install, acc config.Account) error {
	if name, err := platform.RunningProcess(platform.GameExe); err != nil {
		return err
	} else if name != "" {
		return errors.New("the game is already running")
	}
	key, err := sessionFor(client, acc.UserID)
	if err != nil {
		return err
	}
	ticket, err := vkauth.RequestGameTicket(client, key)
	if err != nil {
		return err
	}
	pid, err := platform.StartGame(g.Exe, g.Launch.Args(ticket.GameAccount, ticket.Code), g.Root)
	if err != nil {
		return fmt.Errorf("start the game: %w", err)
	}
	fmt.Printf("Game started: %s, PID %d\n", acc.Label(), pid)
	return nil
}
func dropSession(userID int64) error {
	if token, err := config.LoadRefreshToken(userID); err == nil {
		body, _ := json.Marshal(map[string]string{"client_id": vkauth.OAuthClientID, "refresh_token": token})
		if _, err := vkauth.Post(authClient(), "https://o2-ext-ac.vkplay.ru/api/v3/pub/oauth2/drop", "application/json", body, vkauth.BrowserAgent); err != nil {
			fmt.Fprintln(os.Stderr, "Warning: the server did not confirm session revocation:", err)
		}
	}
	return config.ClearRefreshToken(userID)
}
