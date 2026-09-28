package gamefiles

import (
	"reflect"
	"testing"
)

func TestStaleCacheEntriesKeepsOneVersion(t *testing.T) {
	names := []string{
		"payload-440-441", "stage-440-441", "backup-440-441-20260901-100000",
		"payload-441-442", "stage-441-442", "backup-441-442-20260926-114944",
		"payload-442-443", "stage-442-443",
		"notes", "backup-x-y",
	}
	got := staleCacheEntries(names, 442)
	want := []string{"payload-440-441", "stage-440-441", "backup-440-441-20260901-100000", "stage-441-442"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stale = %v, want %v", got, want)
	}
}
