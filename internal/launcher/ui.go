package launcher

import (
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/tokens"
)

var (
	ErrCancelled = gamefiles.ErrDeclined
	ErrNeedLogin = tokens.ErrNeedLogin
)

type UI interface {
	gamefiles.Asker
	Notify(text string)
	Ask(p Prompt) (string, bool)
	VKSignIn(s VKSignIn) (done func())
	Foreground()
}

type PromptKind string

const (
	PromptEmail  PromptKind = "email"
	PromptCode   PromptKind = "code"
	PromptName   PromptKind = "name"
	PromptKey    PromptKind = "key"
	PromptFolder PromptKind = "folder"
	PromptChoice PromptKind = "choice"
)

type Prompt struct {
	Kind     PromptKind `json:"kind"`
	Question string     `json:"question"`
	Folder   string     `json:"folder,omitempty"`
	Branch   string     `json:"branch,omitempty"`
	Suggest  string     `json:"suggest,omitempty"`
	Options  []Choice   `json:"options,omitempty"`
}

type Choice struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	Detail  string `json:"detail,omitempty"`
	Current bool   `json:"current,omitempty"`
}

type VKSignIn struct {
	URL      string
	FreshURL string
	Report   func(code, state string, err error)
}
