package launcher

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/fileutil"
)

func vkVerificationMarker(root string) string {
	return filepath.Join(root, "-gup-", "awlauncher", "vk-verify-needed")
}

type vkVerificationState struct {
	Build int      `json:"build"`
	Files []string `json:"files"`
}

func markFXDirty(root string) error {
	state, ok := readBranchState(root)
	if !ok || state.Branch != fxDefaultBranch {
		return nil
	}
	state.Dirty = true
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return fileutil.WriteAtomic(branchStatePath(root), data)
}

func markVKDirtyIfSharedFiles(root string, changed []string) error {
	if !isGameDir(root) || len(changed) == 0 {
		return nil
	}
	build, _, err := currentBuild(root)
	if err != nil {
		return err
	}
	inv, err := loadClientInventory(root, build)
	if err != nil {
		for _, name := range changed {
			if !strings.EqualFold(normalizedClientName(name), "user.cfg") {
				return fileutil.WriteAtomic(vkVerificationMarker(root), []byte("1"))
			}
		}
		return nil
	}
	listed := map[string]bool{}
	for _, f := range inv.Files {
		listed[normalizedClientName(f.Name)] = true
	}
	state, full := readVKVerification(root, build)
	if full {
		return nil
	}
	marked := map[string]bool{}
	for _, name := range state.Files {
		marked[name] = true
	}
	for _, name := range changed {
		key := normalizedClientName(name)
		if key != "user.cfg" && listed[key] {
			marked[key] = true
		}
	}
	if len(marked) == 0 {
		return nil
	}
	state.Build = build
	state.Files = state.Files[:0]
	for name := range marked {
		state.Files = append(state.Files, name)
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return fileutil.WriteAtomic(vkVerificationMarker(root), data)
}

func readVKVerification(root string, build int) (vkVerificationState, bool) {
	data, err := os.ReadFile(vkVerificationMarker(root))
	if errors.Is(err, os.ErrNotExist) {
		return vkVerificationState{}, false
	}
	if err != nil {
		return vkVerificationState{}, true
	}
	var state vkVerificationState
	if json.Unmarshal(data, &state) != nil || state.Build != build || len(state.Files) == 0 {
		return vkVerificationState{}, true
	}
	return state, false
}

func vkNeedsFullVerification(root string) bool {
	_, err := os.Stat(vkVerificationMarker(root))
	return err == nil
}

func clearVKVerification(root string) error {
	err := os.Remove(vkVerificationMarker(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
