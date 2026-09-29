package launcher

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/vkplay"
)

const vkSignInTimeout = 15 * time.Minute

func (s *Session) loginVK(name string) (config.Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), vkSignInTimeout)
	defer cancel()
	code, err := s.browserCode(ctx)
	if errors.Is(err, context.DeadlineExceeded) {
		return config.Account{}, fmt.Errorf("sign-in not completed within %s", vkSignInTimeout)
	}
	if err != nil {
		return config.Account{}, err
	}
	userID, err := vkplay.SignIn(s.client, code)
	if err != nil {
		return config.Account{}, err
	}
	acc, err := s.cfg.SignedIn(config.Account{UserID: userID, Name: name})
	s.ui.Sayf("Signed in: %s", acc.Label())
	s.ui.Foreground()
	return acc, err
}
