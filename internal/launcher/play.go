package launcher

import (
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
)

func (s *Session) Play(acc config.Account) error {
	return s.withLogin(acc, func(acc config.Account) error {
		if err := s.play(acc); err != nil {
			return err
		}
		return s.Pin(acc)
	})
}

func (s *Session) play(acc config.Account) error {
	if acc.IsFX() {
		return s.playFX(acc)
	}
	return s.playVK(acc)
}
