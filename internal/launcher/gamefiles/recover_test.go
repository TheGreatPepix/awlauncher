package gamefiles

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, root, name, data string) string {
	t.Helper()
	path, err := SafePath(root, name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readTestFile(t *testing.T, root, name string) string {
	t.Helper()
	path, _ := SafePath(root, name)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "<missing>"
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRecoverInstallAfterCrash(t *testing.T) {
	root := t.TempDir()
	game, stage, backup := filepath.Join(root, "game"), filepath.Join(root, "stage"), filepath.Join(root, "backup")
	last := `<Manifest Build="441"><Misc/><RunCheck/></Manifest>`
	lastName := filepath.Join("-gup-", "last.xml")
	writeTestFile(t, game, lastName, last)
	for _, name := range []string{"a.dat", "b.dat", "d.dat"} {
		writeTestFile(t, game, name, "old "+name)
		writeTestFile(t, stage, name, "new "+name)
	}
	writeTestFile(t, stage, "c.dat", "new c.dat")
	names := []string{"a.dat", "c.dat", "d.dat", "b.dat", lastName}
	if err := writeJournal(game, installJournal{Source: 441, Destination: 442, Stage: stage, Backup: backup, Names: names}); err != nil {
		t.Fatal(err)
	}
	// The launcher stopped after a.dat and c.dat were installed and d.dat was
	// moved to the backup, but before the new d.dat took its place.
	for _, name := range []string{"a.dat", "d.dat"} {
		target, _ := SafePath(game, name)
		bak, _ := SafePath(backup, name)
		if err := moveFile(target, bak); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"a.dat", "c.dat"} {
		src, _ := SafePath(stage, name)
		target, _ := SafePath(game, name)
		if err := moveFile(src, target); err != nil {
			t.Fatal(err)
		}
	}

	if err := RecoverInstall(game); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"a.dat": "old a.dat", "b.dat": "old b.dat", "c.dat": "<missing>", "d.dat": "old d.dat", lastName: last} {
		if got := readTestFile(t, game, name); got != want {
			t.Errorf("game %s = %q, want %q", name, got, want)
		}
	}
	for _, name := range []string{"a.dat", "b.dat", "c.dat", "d.dat"} {
		if got, want := readTestFile(t, stage, name), "new "+name; got != want {
			t.Errorf("stage %s = %q, want %q", name, got, want)
		}
	}
	if _, err := os.Stat(journalPath(game)); !os.IsNotExist(err) {
		t.Fatalf("journal left behind: %v", err)
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatalf("backup left behind: %v", err)
	}
}

func TestRecoverInstallKeepsFinishedInstall(t *testing.T) {
	root := t.TempDir()
	game, backup := filepath.Join(root, "game"), filepath.Join(root, "backup")
	writeTestFile(t, game, filepath.Join("-gup-", "last.xml"), `<Manifest Build="442"><Misc/><RunCheck/></Manifest>`)
	writeTestFile(t, game, "a.dat", "new a.dat")
	writeTestFile(t, backup, "a.dat", "old a.dat")
	if err := writeJournal(game, installJournal{Source: 441, Destination: 442, Stage: filepath.Join(root, "stage"), Backup: backup, Names: []string{"a.dat"}}); err != nil {
		t.Fatal(err)
	}
	if err := RecoverInstall(game); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, game, "a.dat"); got != "new a.dat" {
		t.Fatalf("a.dat = %q", got)
	}
	if _, err := os.Stat(journalPath(game)); !os.IsNotExist(err) {
		t.Fatalf("journal left behind: %v", err)
	}
}

func TestRecoverInstallWithoutJournal(t *testing.T) {
	if err := RecoverInstall(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
