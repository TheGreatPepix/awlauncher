package gamefiles

import (
	"fmt"
)

func repairClientFiles(g Install, damaged []inventoryFile) error {
	if err := EnsureGameClosed(); err != nil {
		return err
	}
	d, err := fetchVKDistrib(g.Build)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(damaged))
	for _, f := range damaged {
		names = append(names, f.Name)
	}
	set, err := d.files(names)
	if err != nil {
		return err
	}
	if _, err := syncFiles(g.Root, set, syncOptions{Jobs: 3}); err != nil {
		return err
	}
	fmt.Println("Client repair complete; downloaded files match the official manifest.")
	return nil
}
