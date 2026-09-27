package region

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TheGreatPepix/awlauncher/internal/fileutil"
)

const (
	VK = "vkplay"
	FX = "fxid"
)

type paths struct {
	dir, marker, vk, fx, root string
}

func pathsOf(root string) paths {
	dir := filepath.Join(root, "-gup-", "awlauncher")
	return paths{
		dir:    dir,
		marker: filepath.Join(dir, "region"),
		vk:     filepath.Join(dir, "user.vkplay.cfg"),
		fx:     filepath.Join(dir, "user.fxid.cfg"),
		root:   filepath.Join(root, "user.cfg"),
	}
}

func FXConfigPath(root string) string { return pathsOf(root).fx }

func Current(root string) string {
	if data, err := os.ReadFile(pathsOf(root).marker); err == nil && strings.TrimSpace(string(data)) == FX {
		return FX
	}
	return VK
}

func Ensure(root, service string) error {
	switch service {
	case FX:
		return toFX(pathsOf(root), Current(root) == FX)
	case VK:
		if Current(root) == FX {
			return toVK(pathsOf(root))
		}
		return nil
	}
	return fmt.Errorf("unknown service %q", service)
}

func toFX(p paths, already bool) error {
	fx, err := os.ReadFile(p.fx)
	if err != nil {
		return fmt.Errorf("the FX ID user.cfg has not been downloaded yet: %w", err)
	}
	current, err := os.ReadFile(p.root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if !already {
		if err == nil && !bytes.Equal(current, fx) {
			if err := fileutil.WriteAtomic(p.vk, current); err != nil {
				return err
			}
		}
		if err := fileutil.WriteAtomic(p.marker, []byte(FX)); err != nil {
			return err
		}
	}
	if err == nil && bytes.Equal(current, fx) {
		return nil
	}
	return fileutil.WriteAtomic(p.root, fx)
}

func toVK(p paths) error {
	vk, err := os.ReadFile(p.vk)
	switch {
	case err == nil:
		if err := fileutil.WriteAtomic(p.root, vk); err != nil {
			return err
		}
	case errors.Is(err, os.ErrNotExist):
		if err := os.Remove(p.root); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	default:
		return err
	}
	if err := os.Remove(p.marker); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Remove(p.vk); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
