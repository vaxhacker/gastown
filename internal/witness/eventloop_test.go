package witness

import (
	"testing"
)

func TestExtractPolecatNameFromActor(t *testing.T) {
	tests := []struct {
		actor string
		want  string
	}{
		{"gastown/polecats/nux", "nux"},
		{"gastown/polecats/alpha", "alpha"},
		{"beads/polecats/Toast", "Toast"},
		{"gastown/witness", ""},
		{"gastown/refinery", ""},
		{"mayor", ""},
		{"deacon", ""},
		{"", ""},
		{"gastown/crew/joe", ""},
	}

	for _, tt := range tests {
		t.Run(tt.actor, func(t *testing.T) {
			got := extractPolecatNameFromActor(tt.actor)
			if got != tt.want {
				t.Errorf("extractPolecatNameFromActor(%q) = %q, want %q", tt.actor, got, tt.want)
			}
		})
	}
}
