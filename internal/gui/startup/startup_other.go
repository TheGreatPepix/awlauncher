//go:build !windows

package startup

import "errors"

func Mode() string { return "off" }

func Set(mode string) error {
	if mode == "off" {
		return nil
	}
	return errors.New("not supported on this system")
}

func Refresh() {}

func TrayRequested([]string) bool { return false }
