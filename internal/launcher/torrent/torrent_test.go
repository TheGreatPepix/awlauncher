package torrent

import (
	"crypto/sha1"
	"fmt"
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

func TestBadFilesOverManyPieces(t *testing.T) {
	root := t.TempDir()
	meta := Meta{PieceSize: 64}
	var stream []byte
	var names []string
	for i := range 60 {
		if i == 30 {
			meta.Files = append(meta.Files, File{Name: "pad", Size: 37, Padding: true})
			stream = append(stream, make([]byte, 37)...)
		}
		name := fmt.Sprintf("f%02d.bin", i)
		data := make([]byte, 1+(i*53)%211)
		for j := range data {
			data[j] = byte(i*7 + j)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0644); err != nil {
			t.Fatal(err)
		}
		meta.Files = append(meta.Files, File{Name: name, Size: int64(len(data))})
		names = append(names, name)
		stream = append(stream, data...)
	}
	for i := 0; i < len(stream); i += 64 {
		sum := sha1.Sum(stream[i:min(i+64, len(stream))])
		meta.Hashes = append(meta.Hashes, sum[:]...)
	}
	for _, name := range []string{"f03.bin", "f31.bin", "f59.bin"} {
		path := filepath.Join(root, name)
		data, _ := os.ReadFile(path)
		data[len(data)/2] ^= 0xff
		os.WriteFile(path, data, 0644)
	}
	var want []string
	var disk []byte
	for _, f := range meta.Files {
		if f.Padding {
			disk = append(disk, make([]byte, f.Size)...)
			continue
		}
		data, _ := os.ReadFile(filepath.Join(root, f.Name))
		disk = append(disk, data...)
	}
	seen := map[string]bool{}
	for p := 0; p*64 < len(disk); p++ {
		start, end := p*64, min(p*64+64, len(disk))
		if sum := sha1.Sum(disk[start:end]); reflect.DeepEqual(sum[:], meta.Hashes[p*20:p*20+20]) {
			continue
		}
		var off int
		for _, f := range meta.Files {
			if !f.Padding && off < end && start < off+int(f.Size) && !seen[f.Name] {
				seen[f.Name] = true
				want = append(want, f.Name)
			}
			off += int(f.Size)
		}
	}
	bad, err := meta.badFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(bad, want) || len(want) == 0 {
		t.Fatalf("bad = %v, want %v", bad, want)
	}

	os.WriteFile(filepath.Join(root, names[10]), []byte("short"), 0644)
	if _, err := meta.badFiles(root); err == nil {
		t.Fatal("a truncated file passed verification")
	}
}
