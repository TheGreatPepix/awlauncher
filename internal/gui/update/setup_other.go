//go:build !windows

package update

import (
	"errors"
	"net/http"
)

var errNoSetup = errors.New("AWLauncher cannot install updates on this system")

func DownloadSetup(*http.Client, string) (string, error) { return "", errNoSetup }
func RunSetup(string, string, bool) error                { return errNoSetup }
func RemoveLeftovers(string)                             {}
