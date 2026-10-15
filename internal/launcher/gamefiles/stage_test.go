package gamefiles

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-deltasync/vcdiff"
)

const stageFixtureDeltas = 12

// stageFixtureFile returns deterministic, compressible file contents.
func stageFixtureFile(seed int64, size int) []byte {
	r := rand.New(rand.NewSource(seed))
	words := []string{"tank ", "armor ", "turret ", "mesh ", "texture ", "shader ", "0x1f "}
	out := make([]byte, 0, size)
	for len(out) < size {
		out = append(out, words[r.Intn(len(words))]...)
	}
	return out[:size]
}

func stageFixtureTarget(i int) []byte {
	t := stageFixtureFile(int64(i), 96<<10)
	copy(t[1000*(i+1):], fmt.Sprintf("changed in build 2, file %d", i))
	return append(t, stageFixtureFile(int64(100+i), 4<<10)...)
}

func stageFixtureManifest() Manifest {
	var m Manifest
	sum := func(data []byte) string { s := md5.Sum(data); return hex.EncodeToString(s[:]) }
	for _, name := range []string{"new/one.dat", "new/two.dat"} {
		data := stageFixtureFile(int64(len(name))*7, 64<<10)
		m.Files.Files = append(m.Files.Files, ManifestFile{Name: name, Size: int64(len(data)), MD5: sum(data)})
	}
	for i := 0; i < stageFixtureDeltas; i++ {
		data := stageFixtureTarget(i)
		m.PatchFiles.Files = append(m.PatchFiles.Files, ManifestFile{Name: fmt.Sprintf("bin\\file%02d.pak", i), Size: int64(len(data)), MD5: sum(data)})
	}
	return m
}

// TestGenerateStageFixture writes the inputs of testdata/stage; run it with
// AWLAUNCHER_STAGE_FIXTURE=<dir>, then pack <dir>/app into app.7z.001 and
// <dir>/patch into patch.7z.001 as solid 7z archives.
func TestGenerateStageFixture(t *testing.T) {
	dir := os.Getenv("AWLAUNCHER_STAGE_FIXTURE")
	if dir == "" {
		t.Skip("fixture generation only")
	}
	m := stageFixtureManifest()
	for _, f := range m.Files.Files {
		path := filepath.Join(dir, "app", filepath.FromSlash(f.Name))
		os.MkdirAll(filepath.Dir(path), 0755)
		if err := os.WriteFile(path, stageFixtureFile(int64(len(f.Name))*7, 64<<10), 0644); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll(filepath.Join(dir, "patch"), 0755)
	for i := 0; i < stageFixtureDeltas; i++ {
		delta := vcdiff.EncodeBytes(stageFixtureFile(int64(i), 96<<10), stageFixtureTarget(i))
		if err := os.WriteFile(filepath.Join(dir, "patch", fmt.Sprint(i)), delta, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func writeStageGame(t *testing.T) string {
	game := t.TempDir()
	for i := 0; i < stageFixtureDeltas; i++ {
		path, _ := SafePath(game, fmt.Sprintf("bin\\file%02d.pak", i))
		os.MkdirAll(filepath.Dir(path), 0755)
		if err := os.WriteFile(path, stageFixtureFile(int64(i), 96<<10), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return game
}

func TestStagePatch(t *testing.T) {
	m := stageFixtureManifest()
	for _, tc := range []struct {
		jobs   int
		budget int64
	}{{1, 1}, {4, 1 << 30}} {
		game := writeStageGame(t)
		stage := t.TempDir()
		names, err := stagePatch(game, filepath.Join("testdata", "stage"), stage, m, tc.jobs, tc.budget)
		if err != nil {
			t.Fatal(err)
		}
		if len(names) != len(m.Files.Files)+len(m.PatchFiles.Files) {
			t.Fatalf("names %v", names)
		}
		for _, f := range append(m.Files.Files, m.PatchFiles.Files...) {
			out, _ := SafePath(stage, f.Name)
			if ok, err := fileMatches(out, f.Size, f.MD5, md5.New, nil); err != nil || !ok {
				t.Fatalf("jobs %d: %s not built (%v)", tc.jobs, f.Name, err)
			}
		}
		// A second run keeps the verified files and builds nothing.
		if _, err := stagePatch(game, filepath.Join("testdata", "stage"), stage, m, tc.jobs, tc.budget); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStagePatchBadDelta(t *testing.T) {
	m := stageFixtureManifest()
	m.PatchFiles.Files[5].MD5 = "00000000000000000000000000000000"
	_, err := stagePatch(writeStageGame(t), filepath.Join("testdata", "stage"), t.TempDir(), m, 3, 1<<30)
	if err == nil {
		t.Fatal("a wrong MD5 was accepted")
	}
}
