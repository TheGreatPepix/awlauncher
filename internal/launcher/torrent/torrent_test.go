package torrent

import (
	"crypto/sha1"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBadFilesFindsCorruptFile(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{"a.bin": "0123456789", "b.bin": "abcdefghij", "c.bin": "ABCDEFGHIJ"}
	meta := Meta{PieceSize: 8}
	var stream string
	for _, name := range []string{"a.bin", "b.bin", "c.bin"} {
		meta.Files = append(meta.Files, File{Name: name, Size: int64(len(files[name]))})
		stream += files[name]
		if err := os.WriteFile(filepath.Join(root, name), []byte(files[name]), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < len(stream); i += 8 {
		end := min(i+8, len(stream))
		sum := sha1.Sum([]byte(stream[i:end]))
		meta.Hashes = append(meta.Hashes, sum[:]...)
	}
	if bad, err := meta.badFiles(root); err != nil || len(bad) != 0 {
		t.Fatalf("clean: %v, %v", bad, err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.bin"), []byte("abXdefghij"), 0644); err != nil {
		t.Fatal(err)
	}
	bad, err := meta.badFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.bin", "b.bin"}; !reflect.DeepEqual(bad, want) {
		t.Fatalf("bad = %v, want %v", bad, want)
	}
	if meta.verifyPieces(root) == nil {
		t.Fatal("verifyPieces accepted a corrupt file")
	}
}
