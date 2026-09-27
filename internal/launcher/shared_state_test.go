package launcher

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"github.com/TheGreatPepix/awlauncher/internal/region"
	"os"
	"path/filepath"
	"testing"
)

func TestSharedStateInvalidatesOnlyCommonFiles(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string][]byte{
		"-gup-/last.xml": []byte(`<Manifest Build="442"/>`),
		"bin64/game.exe": []byte("game"),
	})
	inv := clientInventory{Build: 442, Files: []inventoryFile{{Name: "bin64/game.exe", Size: 4}, {Name: "user.cfg", Size: 8}}}
	if err := saveClientInventory(root, inv); err != nil {
		t.Fatal(err)
	}
	if err := markVKDirtyIfSharedFiles(root, []string{"user.cfg", "fx-only.pak"}); err != nil {
		t.Fatal(err)
	}
	if vkNeedsFullVerification(root) {
		t.Fatal("service-specific files triggered a full VK Play check")
	}
	if err := markVKDirtyIfSharedFiles(root, []string{"BIN64\\GAME.EXE"}); err != nil {
		t.Fatal(err)
	}
	if !vkNeedsFullVerification(root) {
		t.Fatal("shared file did not trigger a full VK Play check")
	}
	state, full := readVKVerification(root, 442)
	if full || len(state.Files) != 1 || state.Files[0] != "bin64/game.exe" {
		t.Fatalf("wrong targeted VK check: %+v, full=%v", state, full)
	}
	if err := clearVKVerification(root); err != nil || vkNeedsFullVerification(root) {
		t.Fatal("VK Play check marker was not cleared", err)
	}
}

func TestSharedFolderGetsEachServicesUserCfg(t *testing.T) {
	root := t.TempDir()
	vk1 := "net_frontline_address = vk.example\nvk_only = first\n"
	writeTree(t, root, map[string][]byte{
		"-gup-/last.xml": []byte(`<Manifest Build="442"/>`),
		"user.cfg":       []byte(vk1),
	})
	b := newBranchServer(t)
	fx1 := "net_frontline_address = 95.211.7.247\nfx_only = first\n"
	b.files = map[string][]byte{"user.cfg": []byte(fx1)}
	sync := func() {
		t.Helper()
		if _, err := syncBranch(b.srv.Client(), fxDefaultBranch, b.branchManifest(), "", root); err != nil {
			t.Fatal(err)
		}
	}
	use := func(service, want string) {
		t.Helper()
		if err := region.Ensure(root, service); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, filepath.Join(root, "user.cfg")); got != want {
			t.Fatalf("%s: user.cfg = %q, want %q", service, got, want)
		}
	}
	sync()
	if got := readFile(t, filepath.Join(root, "user.cfg")); got != vk1 {
		t.Fatalf("the sync replaced the VK Play user.cfg: %q", got)
	}
	use(region.FX, fx1)
	use(region.VK, vk1)
	vk2 := "net_frontline_address = vk.example\nvk_only = second\n"
	if err := os.WriteFile(filepath.Join(root, "user.cfg"), []byte(vk2), 0644); err != nil {
		t.Fatal(err)
	}
	fx2 := "net_frontline_address = 95.211.7.247\nfx_only = second\n"
	b.files["user.cfg"] = []byte(fx2)
	sync()
	use(region.FX, fx2)
	use(region.VK, vk2)
}

func TestFXDirtyRechecksSameSizeContent(t *testing.T) {
	root := t.TempDir()
	b := newBranchServer(t)
	b.files = map[string][]byte{"bin64/game.exe": []byte("good")}
	state, err := syncBranch(b.srv.Client(), fxDefaultBranch, b.branchManifest(), "", root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin64", "game.exe"), []byte("evil"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := markFXDirty(root); err != nil {
		t.Fatal(err)
	}
	state, err = syncBranch(b.srv.Client(), fxDefaultBranch, b.branchManifest(), "", root)
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(root, "bin64", "game.exe")); got != "good" {
		t.Fatalf("same-size content was trusted: %q", got)
	}
	if state.Dirty || len(state.Changed) != 1 || state.Changed[0] != "bin64/game.exe" {
		t.Fatalf("state after repair: %+v", state)
	}
}

func TestMismatchedVKFilesChecksMD5(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string][]byte{"good.dat": []byte("good"), "bad.dat": []byte("evil")})
	digest := func(s string) string { sum := md5.Sum([]byte(s)); return hex.EncodeToString(sum[:]) }
	manifest := patchManifest{}
	manifest.NonCompressed.Files = []manifestFile{
		{Name: "good.dat", Size: 4, MD5: digest("good")},
		{Name: "bad.dat", Size: 4, MD5: digest("good")},
		{Name: "missing.dat", Size: 2, MD5: digest("ok")},
	}
	bad, err := mismatchedVKFiles(root, manifest)
	if err != nil || len(bad) != 2 || bad[0].Name != "bad.dat" || bad[1].Name != "missing.dat" {
		t.Fatalf("mismatched files = %v, err = %v", bad, err)
	}
}

func TestVKModModeSkipsExistingFilesButNotMissing(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string][]byte{"hangar.pak": []byte("modded")})
	manifest := patchManifest{}
	manifest.NonCompressed.Files = []manifestFile{
		{Name: "hangar.pak", Size: 4, MD5: "00000000000000000000000000000000"},
		{Name: "missing.pak", Size: 4, MD5: "00000000000000000000000000000000"},
	}
	bad, err := mismatchedVKFilesSelectedWithMods(root, manifest, nil, true, true)
	if err != nil || len(bad) != 1 || bad[0].Name != "missing.pak" {
		t.Fatalf("with mods: bad=%v, err=%v", bad, err)
	}
}

func TestVKVerificationOnlyChecksChangedFiles(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string][]byte{"changed.dat": []byte("evil"), "untouched.dat": []byte("evil")})
	digest := func(s string) string { sum := md5.Sum([]byte(s)); return hex.EncodeToString(sum[:]) }
	manifest := patchManifest{}
	manifest.NonCompressed.Files = []manifestFile{
		{Name: "changed.dat", Size: 4, MD5: digest("good")},
		{Name: "untouched.dat", Size: 4, MD5: digest("good")},
	}
	bad, err := mismatchedVKFilesSelected(root, manifest, []string{"changed.dat"}, false)
	if err != nil || len(bad) != 1 || bad[0].Name != "changed.dat" {
		t.Fatalf("targeted check = %v, err = %v", bad, err)
	}
	bad, err = mismatchedVKFilesSelected(root, manifest, []string{"unknown.dat"}, false)
	if err != nil || len(bad) != 2 {
		t.Fatalf("unknown name should trigger complete check: %v, err = %v", bad, err)
	}
}

func TestVKVerificationCombinesChangesAndPreservesLegacyMarker(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string][]byte{"-gup-/last.xml": []byte(`<Manifest Build="442"/>`)})
	inv := clientInventory{Build: 442, Files: []inventoryFile{{Name: "a.dat"}, {Name: "b.dat"}}}
	if err := saveClientInventory(root, inv); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.dat", "b.dat", "a.dat"} {
		if err := markVKDirtyIfSharedFiles(root, []string{name}); err != nil {
			t.Fatal(err)
		}
	}
	state, full := readVKVerification(root, 442)
	if full || len(state.Files) != 2 {
		t.Fatalf("changes were not combined: %+v, full=%v", state, full)
	}
	if err := os.WriteFile(vkVerificationMarker(root), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := markVKDirtyIfSharedFiles(root, []string{"a.dat"}); err != nil {
		t.Fatal(err)
	}
	if _, full := readVKVerification(root, 442); !full {
		t.Fatal("legacy marker was downgraded to a partial check")
	}
}

func TestMarkFXDirtyPersists(t *testing.T) {
	root := t.TempDir()
	path := branchStatePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(branchState{Branch: fxDefaultBranch, Files: []fxFile{{Path: "file"}}})
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := markFXDirty(root); err != nil {
		t.Fatal(err)
	}
	state, ok := readBranchState(root)
	if !ok || !state.Dirty {
		t.Fatalf("FX state was not marked dirty: %+v", state)
	}
}
