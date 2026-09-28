package launcher

import (
	"errors"
	"fmt"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
)

func (s *Session) ChooseBranch(acc config.Account) error {
	if !acc.IsFX() {
		return errors.New("branches are available only to FX ID accounts")
	}
	return s.withLogin(acc, func(acc config.Account) error {
		s.ui.Sayf("Asking FX ID for branches of %s...", acc.Label())
		branches, err := fxBranches(s.client, acc)
		if err != nil {
			return fmt.Errorf("could not list branches: %w", err)
		}
		logFXBranches(branches)
		return s.chooseBranch(acc, branches)
	})
}

func (s *Session) chooseBranch(acc config.Account, branches []fxBranchInfo) error {
	current := acc.Branch
	if current == "" {
		current = gamefiles.DefaultBranch
	}
	var options []Choice
	for _, b := range branches {
		if b.Err == nil {
			options = append(options, Choice{Value: b.Name, Label: b.Name, Detail: b.describe(), Current: strings.EqualFold(b.Name, current)})
		}
	}
	if len(options) == 0 {
		s.ui.Notify("No branches are available to this account")
		return nil
	}
	answer, ok := s.ui.Ask(Prompt{Kind: PromptChoice, Question: "Which branch should " + acc.Label() + " play?", Options: options})
	if !ok || answer == "" || strings.EqualFold(answer, current) {
		return nil
	}
	chosen := ""
	for _, o := range options {
		if o.Value == answer {
			chosen = o.Value
		}
	}
	if chosen == "" {
		return errors.New("that branch is not available to this account")
	}
	if chosen == gamefiles.DefaultBranch {
		chosen = ""
	}
	if err := s.cfg.UpdateAccount(acc.UserID, func(a *config.Account) { a.Branch = chosen }); err != nil {
		return err
	}
	if chosen == "" {
		s.ui.Say("The account will play the main client.")
	} else {
		s.ui.Sayf("The account will play %s. It is installed next to the main game on the next start.", chosen)
	}
	return nil
}

func (s *Session) ActivateKey(acc config.Account) error {
	if !acc.IsFX() {
		return errors.New("keys are activated only on FX ID accounts")
	}
	key, ok := s.ui.Ask(Prompt{Kind: PromptKey, Question: "Key for a closed branch"})
	if key = strings.TrimSpace(key); !ok || key == "" {
		return ErrCancelled
	}
	return s.withLogin(acc, func(acc config.Account) error {
		branch, err := fxActivateKey(s.client, acc, key)
		if err != nil {
			return fmt.Errorf("key not activated: %w", err)
		}
		s.ui.Sayf("Key activated: branch %q is now available to %s.", branch, acc.Label())
		branches, err := fxBranches(s.client, acc)
		if err != nil {
			return fmt.Errorf("could not list branches: %w", err)
		}
		logFXBranches(branches)
		return s.chooseBranch(acc, branches)
	})
}
