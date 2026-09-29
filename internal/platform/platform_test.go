package platform

import (
	"slices"
	"testing"
)

func TestGameEnvReplacesGCVars(t *testing.T) {
	base := []string{"PATH=x", "GC_PERS_ID=old", "gc_project_id=11321", "GCX=keep"}
	got := gameEnv(base, []string{"GC_PERS_ID=new"})
	want := []string{"PATH=x", "GCX=keep", "GC_PERS_ID=new"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
