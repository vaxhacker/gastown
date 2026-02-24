package cmd

import "testing"

func TestExpandMailRoleShortcut(t *testing.T) {
	tests := []struct {
		name string
		to   string
		from string
		want string
	}{
		{
			name: "librarian slash from polecat",
			to:   "librarian/",
			from: "gastown/nux",
			want: "gastown/librarian",
		},
		{
			name: "librarian bare from crew",
			to:   "librarian",
			from: "gastown/crew/max",
			want: "gastown/librarian",
		},
		{
			name: "witness shortcut",
			to:   "witness/",
			from: "beads/refinery",
			want: "beads/witness",
		},
		{
			name: "already rig qualified unchanged",
			to:   "gastown/librarian",
			from: "gastown/nux",
			want: "gastown/librarian",
		},
		{
			name: "non-shortcut unchanged",
			to:   "mayor/",
			from: "gastown/nux",
			want: "mayor/",
		},
		{
			name: "fallback to role context when sender has no rig",
			to:   "librarian/",
			from: "overseer",
			want: "gastown/librarian",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandMailRoleShortcut(tt.to, tt.from)
			if got != tt.want {
				t.Fatalf("expandMailRoleShortcut(%q, %q) = %q, want %q", tt.to, tt.from, got, tt.want)
			}
		})
	}
}

func TestRigFromIdentity(t *testing.T) {
	tests := []struct {
		identity string
		want     string
	}{
		{identity: "gastown/nux", want: "gastown"},
		{identity: "gastown/crew/max", want: "gastown"},
		{identity: "gastown/witness", want: "gastown"},
		{identity: "overseer", want: ""},
		{identity: "mayor/", want: ""},
		{identity: "deacon/", want: ""},
		{identity: "", want: ""},
	}

	for _, tt := range tests {
		got := rigFromIdentity(tt.identity)
		if got != tt.want {
			t.Fatalf("rigFromIdentity(%q) = %q, want %q", tt.identity, got, tt.want)
		}
	}
}
