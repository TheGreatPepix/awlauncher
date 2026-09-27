package launcher

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestSeedVKFromFXReusesOnlyMatchingFiles(t *testing.T) {
	root := t.TempDir()
	fx := filepath.Join(root, "fx")
	vk := filepath.Join(root, "vk")
	if err := os.MkdirAll(filepath.Join(fx, "bin64"), 0755); err != nil {
		t.Fatal(err)
	}
	large := bytes.Repeat([]byte("x"), linkMinSize)
	files := map[string][]byte{
		"bin64/large.pak": large,
		"bin64/small.cfg": []byte("config"),
		"bin64/wrong.dll": []byte("wrong"),
		"user.cfg":        []byte("FX config"),
	}
	for name, data := range files {
		path := filepath.Join(fx, filepath.FromSlash(name))
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	item := func(name string, data []byte) manifestFile {
		sum := md5.Sum(data)
		return manifestFile{Name: name, Size: int64(len(data)), MD5: hex.EncodeToString(sum[:])}
	}
	manifest := patchManifest{NonCompressed: fileGroup{Files: []manifestFile{
		item("bin64/large.pak", large),
		item("bin64/small.cfg", []byte("config")),
		item("bin64/wrong.dll", []byte("right")),
		item("user.cfg", []byte("VK config")),
	}}}
	linked, copied, err := seedVKFromFX(vk, fx, manifest)
	if err != nil || linked != int64(len(large)) || copied != 6 {
		t.Fatalf("linked=%d copied=%d err=%v", linked, copied, err)
	}
	largeFX, _ := os.Stat(filepath.Join(fx, "bin64", "large.pak"))
	largeVK, _ := os.Stat(filepath.Join(vk, "bin64", "large.pak"))
	if largeFX == nil || largeVK == nil || !os.SameFile(largeFX, largeVK) {
		t.Fatal("matching large file was not hard-linked")
	}
	if _, err := os.Stat(filepath.Join(vk, "bin64", "wrong.dll")); !os.IsNotExist(err) {
		t.Fatalf("wrong file was reused: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vk, "user.cfg")); !os.IsNotExist(err) {
		t.Fatalf("FX user.cfg was reused: %v", err)
	}
}
