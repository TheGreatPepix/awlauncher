package gamefiles

import (
	"reflect"
	"testing"
)

func TestStaleCacheEntriesKeepsOneVersion(t *testing.T) {
	names := []string{
		"patch-440-441", "stage-440-441", "backup-440-441-20260901-100000",
		"patch-441-442", "stage-441-442", "backup-441-442-20260926-114944",
		"patch-442-443", "stage-442-443",
		"notes", "backup-x-y",
	}
	got := staleCacheEntries(names, 442)
	want := []string{"patch-440-441", "stage-440-441", "backup-440-441-20260901-100000", "stage-441-442"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stale = %v, want %v", got, want)
	}
}
