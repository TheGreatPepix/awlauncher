package launcher

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/config"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/fxid"
	"github.com/TheGreatPepix/awlauncher/internal/launcher/gamefiles"
)

func TestOperationsRunTogetherUnlessTheyConflict(t *testing.T) {
	var ops Ops
	install := Operation{Title: "Starting A", Game: true, Account: "1"}
	if reason := ops.Begin(&install); reason != "" {
		t.Fatal(reason)
	}
	for _, op := range []Operation{{Title: "Loading branches of B", Account: "2"}, {Title: "Signing in to VK Play"}} {
		if reason := ops.Begin(&op); reason != "" {
			t.Fatalf("%s refused: %s", op.Title, reason)
		}
	}
	for _, op := range []Operation{{Title: "Removing A", Account: "1"}, {Title: "Starting C", Game: true, Account: "3"}} {
		if ops.Begin(&op) == "" {
			t.Fatalf("%s started beside the install", op.Title)
		}
	}
	if !ops.GameBusy() || len(ops.List()) != 3 {
		t.Fatalf("operations = %+v", ops.List())
	}
	if left := ops.Finish(install.ID); left != 2 {
		t.Fatalf("%d operations left", left)
	}
	next := Operation{Title: "Starting C", Game: true, Account: "3"}
	if reason := ops.Begin(&next); reason != "" {
		t.Fatal(reason)
	}
}

func TestChooseBranchOffersTypedOptions(t *testing.T) {
	acc := config.Account{UserID: -5, Provider: config.ProviderFX, Email: "a@b"}
	ui := &fakeUI{answers: []string{"supertest"}}
	s := NewSession(config.NewStore(config.Config{Accounts: []config.Account{acc}})).WithUI(ui)
	branches := []fxid.Branch{testBranch("default", "0.566.1", 7), testBranch("supertest", "0.567.0", 8), {Name: "closed", Err: errors.New("no access")}}
	if err := s.chooseBranch(acc, branches); err != nil {
		t.Fatal(err)
	}
	if len(ui.asked) != 1 {
		t.Fatalf("prompts = %+v", ui.asked)
	}
	p := ui.asked[0]
	if p.Kind != PromptChoice || len(p.Options) != 2 || !p.Options[0].Current || p.Options[1].Current || p.Options[1].Detail != "0.567.0 (build 8)" {
		t.Fatalf("prompt = %+v", p)
	}
	if got, _ := s.cfg.Find(acc.UserID); got.Branch != "supertest" {
		t.Fatalf("branch = %q", got.Branch)
	}
	ui.answers = []string{"default"}
	if err := s.chooseBranch(stored(s, acc), branches); err != nil {
		t.Fatal(err)
	}
	if a := stored(s, acc); a.Branch != "" {
		t.Fatalf("main branch stored as %q", a.Branch)
	}
	if err := s.chooseBranch(acc, branches); err != nil {
		t.Fatal(err)
	}
	if a := stored(s, acc); a.Branch != "" {
		t.Fatal("a cancelled choice changed the branch")
	}
}

func testBranch(name, version string, build int64) fxid.Branch {
	b := fxid.Branch{Name: name}
	b.Manifest.Manifest.Release.BuildVersion = version
	b.Manifest.Manifest.Release.BuildNumber = build
	return b
}

func stored(s *Session, acc config.Account) config.Account {
	a, _ := s.cfg.Find(acc.UserID)
	return a
}

func TestFolderSettersKeepClientsApart(t *testing.T) {
	parent := t.TempDir()
	vk := filepath.Join(parent, "VK")
	fx := filepath.Join(parent, "FX")
	writeTree(t, vk, map[string][]byte{"-gup-/last.xml": []byte(`<Manifest Build="442"/>`)})
	writeBranchState(t, fx, gamefiles.BranchState{Branch: gamefiles.DefaultBranch})
	s := gameSession(t, "", true)

	if err := s.SetVKFolder(fx); err == nil {
		t.Fatal("VK Play folder accepted an FX ID client")
	}
	if err := s.SetVKFolder("relative"); !errors.Is(err, errNotAFolder) {
		t.Fatalf("relative folder: %v", err)
	}
	if err := s.SetVKFolder(vk); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFXFolder(vk); err == nil {
		t.Fatal("FX ID folder accepted the VK Play client")
	}
	if err := s.SetFXFolder(fx); err != nil {
		t.Fatal(err)
	}
	if err := s.SetBranchFolder("supertest", fx); err == nil {
		t.Fatal("a branch was put into the FX ID main folder")
	}
	if err := s.SetBranchFolder("default", filepath.Join(parent, "B")); err == nil {
		t.Fatal("the main branch got a closed branch folder")
	}
	branch := filepath.Join(parent, "SuperTest")
	if err := s.SetBranchFolder("SuperTest", branch); err != nil {
		t.Fatal(err)
	}
	cfg := s.cfg.Get()
	if cfg.Game != vk || cfg.FXGame != fx || cfg.BranchDir("supertest") != branch {
		t.Fatalf("config = %+v", cfg)
	}
	if err := s.SetVKFolder(branch); err == nil {
		t.Fatal("VK Play folder accepted a closed branch folder")
	}
	if p := s.FolderPrompt(gamefiles.KindBranch, "SuperTest"); p.Kind != PromptFolder || p.Suggest != branch || p.Branch != "SuperTest" {
		t.Fatalf("branch prompt = %+v", p)
	}
}
