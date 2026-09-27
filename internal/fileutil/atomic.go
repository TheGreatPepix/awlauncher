package fileutil

import "os"

func WriteAtomic(path string, data []byte) error {
	tmp := path + ".awlauncher.tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
