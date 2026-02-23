package cmd

import "testing"

func TestEnforcerCommandRegistered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "enforcer" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("enforcer command not found on rootCmd")
	}
}

func TestEnforcerSubcommands(t *testing.T) {
	expected := []string{"start", "status", "restart", "stop"}
	for _, name := range expected {
		found := false
		for _, c := range enforcerCmd.Commands() {
			if c.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("subcommand %q not found on enforcer command", name)
		}
	}
}

func TestEnforcerDefaults(t *testing.T) {
	rigFlag := enforcerCmd.PersistentFlags().Lookup("rig")
	if rigFlag == nil || rigFlag.DefValue != "gastown" {
		t.Fatalf("enforcer --rig default = %q, want %q", rigFlag.DefValue, "gastown")
	}

	nameFlag := enforcerCmd.PersistentFlags().Lookup("name")
	if nameFlag == nil || nameFlag.DefValue != "enforcer" {
		t.Fatalf("enforcer --name default = %q, want %q", nameFlag.DefValue, "enforcer")
	}
}
