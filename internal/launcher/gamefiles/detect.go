package gamefiles

import (
	"os"
	"path/filepath"

	"github.com/TheGreatPepix/awlauncher/internal/launcher/platform"
)

func IsVKInstall(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "-gup-", "last.xml"))
	return err == nil
}

var gameDirNames = []string{
	"Games/Armored Warfare", "Games/ArmoredWarfare", "GamesMailRu/Armored Warfare", "VK Play/Armored Warfare",
	"WishlistGames/Armored Warfare", "Wishlist Games/Armored Warfare",
	"Armored Warfare",
}

func DetectVKInstall() string {
	for _, root := range platform.FixedDrives() {
		for _, sub := range gameDirNames {
			if dir := filepath.Join(root, filepath.FromSlash(sub)); IsVKInstall(dir) {
				return dir
			}
		}
	}
	return ""
}
