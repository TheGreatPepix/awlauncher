package launcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/progress"
)

func seedVKFromFX(vkRoot, fxRoot string, manifest patchManifest) (int64, int64, error) {
	var linked, copied int64
	progress.Default.Begin("Checking FX ID files for reuse", progress.UnitFiles, int64(len(manifest.NonCompressed.Files)), 0)
	defer progress.Default.End()
	for _, item := range manifest.NonCompressed.Files {
		if strings.EqualFold(item.Name, "user.cfg") {
			progress.Default.Add(1)
			continue
		}
		src, err := safeGamePath(fxRoot, item.Name)
		if err != nil {
			return linked, copied, err
		}
		dst, err := safeGamePath(vkRoot, item.Name)
		if err != nil {
			return linked, copied, err
		}
		if st, err := os.Stat(dst); err == nil && st.Size() == item.Size {
			progress.Default.Add(1)
			continue
		}
		ok, err := validStagedFile(src, item)
		if err != nil || !ok {
			progress.Default.Add(1)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return linked, copied, err
		}
		if item.Size >= linkMinSize && os.Link(src, dst) == nil {
			linked += item.Size
		} else {
			if err := copyFile(src, dst); err != nil {
				return linked, copied, fmt.Errorf("reuse %s: %w", item.Name, err)
			}
			copied += item.Size
		}
		progress.Default.Add(1)
	}
	return linked, copied, nil
}
