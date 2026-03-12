package cmd

import (
	"reflect"
	"testing"

	"github.com/steveyegge/gastown/internal/reaper"
)

func TestProductionDatabasesUsesReaperDefaults(t *testing.T) {
	got := productionDatabases()
	if !reflect.DeepEqual(got, reaper.DefaultDatabases) {
		t.Fatalf("productionDatabases() = %v, want %v", got, reaper.DefaultDatabases)
	}

	got[0] = "mutated"
	if reflect.DeepEqual(got, reaper.DefaultDatabases) {
		t.Fatal("productionDatabases() should return a copy")
	}
}
