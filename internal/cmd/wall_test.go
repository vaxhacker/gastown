package cmd

import (
	"testing"
)

func TestWallEmptyMessage(t *testing.T) {
	// cobra.ExactArgs(1) handles no-arg case, but empty string should error
	err := runWall(wallCmd, []string{""})
	if err == nil {
		t.Fatal("expected error for empty message")
	}
	if err.Error() != "message cannot be empty" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWallInvalidMode(t *testing.T) {
	origMode := wallMode
	defer func() { wallMode = origMode }()

	wallMode = "bogus"
	err := runWall(wallCmd, []string{"hello"})
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
	if want := `invalid --mode "bogus": must be one of immediate, queue, wait-idle`; err.Error() != want {
		t.Errorf("got error %q, want %q", err.Error(), want)
	}
}

func TestFormatRoleSender(t *testing.T) {
	tests := []struct {
		name string
		role RoleInfo
		want string
	}{
		{
			name: "mayor",
			role: RoleInfo{Role: RoleMayor},
			want: "mayor",
		},
		{
			name: "deacon",
			role: RoleInfo{Role: RoleDeacon},
			want: "deacon",
		},
		{
			name: "polecat",
			role: RoleInfo{Role: RolePolecat, Rig: "gastown", Polecat: "furiosa"},
			want: "gastown/furiosa",
		},
		{
			name: "crew",
			role: RoleInfo{Role: RoleCrew, Rig: "gastown", Polecat: "max"},
			want: "gastown/crew/max",
		},
		{
			name: "witness",
			role: RoleInfo{Role: RoleWitness, Rig: "gastown"},
			want: "gastown/witness",
		},
		{
			name: "refinery",
			role: RoleInfo{Role: RoleRefinery, Rig: "gastown"},
			want: "gastown/refinery",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRoleSender(tt.role)
			if got != tt.want {
				t.Errorf("formatRoleSender() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWallCommandRegistration(t *testing.T) {
	// Verify the wall command is registered on root
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "wall" {
			found = true
			if cmd.GroupID != GroupComm {
				t.Errorf("wall command group = %q, want %q", cmd.GroupID, GroupComm)
			}
			break
		}
	}
	if !found {
		t.Error("wall command not registered on root command")
	}
}

func TestWallDefaultMode(t *testing.T) {
	// Default mode should be queue (non-disruptive)
	cmd := wallCmd
	f := cmd.Flags().Lookup("mode")
	if f == nil {
		t.Fatal("mode flag not found")
	}
	if f.DefValue != NudgeModeQueue {
		t.Errorf("default mode = %q, want %q", f.DefValue, NudgeModeQueue)
	}
}
