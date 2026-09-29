package gui

import (
	"strconv"

	"github.com/TheGreatPepix/awlauncher/internal/gui/startup"
	"github.com/TheGreatPepix/awlauncher/internal/gui/ui"
	"github.com/TheGreatPepix/awlauncher/internal/launcher"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/platform"
)

type uiAccount struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Login    string `json:"login"`
	Service  string `json:"service"`
	Provider string `json:"provider"`
	Branch   string `json:"branch,omitempty"`
	Last     bool   `json:"last"`
}

type uiState struct {
	Type          string               `json:"type"`
	Accounts      []uiAccount          `json:"accounts"`
	Game          string               `json:"game"`
	SuggestedGame string               `json:"suggestedGame"`
	FXGame        string               `json:"fxGame"`
	AllowMods     bool                 `json:"allowMods"`
	PatchBackups  bool                 `json:"patchBackups"`
	Data          string               `json:"data"`
	Version       string               `json:"version"`
	Autostart     string               `json:"autostart"`
	SystemLang    string               `json:"systemLang"`
	Ops           []launcher.Operation `json:"ops"`
	Running       bool                 `json:"running"`
	Log           *string              `json:"log,omitempty"`
	Prefs         *uiPrefs             `json:"prefs,omitempty"`
}

func accountViews(cfg config.Config) []uiAccount {
	views := make([]uiAccount, 0, len(cfg.Accounts))
	for _, a := range cfg.Accounts {
		provider := "vkplay"
		if a.IsFX() {
			provider = config.ProviderFX
		}
		views = append(views, uiAccount{
			ID: strconv.FormatInt(a.UserID, 10), Name: a.DisplayName(), Login: a.Login(), Service: a.Service(),
			Provider: provider, Branch: a.Branch, Last: a.UserID == cfg.LastUserID,
		})
	}
	return views
}

func (g *App) emitState(withLog bool) {
	cfg := g.store.Get()
	s := uiState{
		Type: "state", Accounts: accountViews(cfg), Game: cfg.Game, FXGame: cfg.FXGame, AllowMods: cfg.AllowMods, PatchBackups: !cfg.NoBackups,
		SuggestedGame: launcher.SuggestGameFolder(cfg.Game), Ops: g.ops.List(), Running: g.gameUp.Load(),
		Version: launcher.Version, Autostart: startup.Mode(), SystemLang: platform.Language(),
	}
	if dir, err := platform.DataDir(); err == nil {
		s.Data = dir
	}
	if withLog {
		text := g.logs.String()
		s.Log = &text
		prefs := loadPrefs()
		s.Prefs = &prefs
	}
	g.emit(s)
	g.updateTray()
}

func (g *App) updateTray() {
	acc, ok := g.store.Find(g.store.Get().LastUserID)
	if !ok {
		g.host.setTrayPlay("", false)
		return
	}
	g.host.setTrayPlay(ui.Translate("Play")+" "+acc.DisplayName()+" · "+acc.Service(), !g.ops.GameBusy() && !g.gameUp.Load())
}
