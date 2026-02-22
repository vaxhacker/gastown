package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRequireSubcommandShowsHelpOnMissingSubcommand(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "example",
		Short: "example command",
	}

	if err := requireSubcommand(cmd, nil); err != nil {
		t.Fatalf("requireSubcommand() with no args returned error: %v", err)
	}
}

func TestResolveRigNameOrInferWithExplicitRig(t *testing.T) {
	got, err := resolveRigNameOrInfer("gastown", "")
	if err != nil {
		t.Fatalf("resolveRigNameOrInfer() returned error: %v", err)
	}
	if got != "gastown" {
		t.Fatalf("resolveRigNameOrInfer() = %q, want %q", got, "gastown")
	}
}

func TestResolveRigNameOrInferOutsideWorkspace(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	_, err = resolveRigNameOrInfer("", "gt witness status [rig]")
	if err == nil {
		t.Fatal("resolveRigNameOrInfer() expected error outside workspace, got nil")
	}
	if !strings.Contains(err.Error(), "not in a Gas Town workspace") {
		t.Fatalf("resolveRigNameOrInfer() error = %q, want workspace hint", err.Error())
	}
}

func TestCommandAliasesAndArgValidators(t *testing.T) {
	if !hasAlias(polecatStatusCmd, "show") {
		t.Fatal("expected polecat status command to have alias 'show'")
	}
	if !hasAlias(rigStatusCmd, "show") {
		t.Fatal("expected rig status command to have alias 'show'")
	}
	if !hasAlias(refineryStatusCmd, "log") {
		t.Fatal("expected refinery status command to have alias 'log'")
	}
	if !hasAlias(mqStatusCmd, "show") {
		t.Fatal("expected mq status command to have alias 'show'")
	}
	if !hasAlias(witnessStatusCmd, "show") {
		t.Fatal("expected witness status command to have alias 'show'")
	}

	if err := mqListCmd.Args(mqListCmd, []string{}); err != nil {
		t.Fatalf("mq list should accept zero args (infer rig): %v", err)
	}
	if err := mqListCmd.Args(mqListCmd, []string{"gastown"}); err != nil {
		t.Fatalf("mq list should accept one arg: %v", err)
	}
	if err := mqListCmd.Args(mqListCmd, []string{"gastown", "extra"}); err == nil {
		t.Fatal("mq list should reject more than one arg")
	}

	if err := witnessStatusCmd.Args(witnessStatusCmd, []string{}); err != nil {
		t.Fatalf("witness status should accept zero args (infer rig): %v", err)
	}
	if err := witnessStatusCmd.Args(witnessStatusCmd, []string{"gastown"}); err != nil {
		t.Fatalf("witness status should accept one arg: %v", err)
	}
	if err := witnessStatusCmd.Args(witnessStatusCmd, []string{"gastown", "extra"}); err == nil {
		t.Fatal("witness status should reject more than one arg")
	}
}

func TestBeadListSubcommandIsRegistered(t *testing.T) {
	for _, sub := range beadCmd.Commands() {
		if sub.Name() == "list" {
			return
		}
	}
	t.Fatal("expected bead list subcommand to be registered")
}

func hasAlias(cmd *cobra.Command, want string) bool {
	for _, alias := range cmd.Aliases {
		if alias == want {
			return true
		}
	}
	return false
}
