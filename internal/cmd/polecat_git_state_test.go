package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetGitState_IgnoresBeadsRuntimeFiles(t *testing.T) {
	repo := initPolecatGitStateRepo(t)

	beadsFile := filepath.Join(repo, ".beads", "runtime.txt")
	if err := os.MkdirAll(filepath.Dir(beadsFile), 0755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	if err := os.WriteFile(beadsFile, []byte("runtime"), 0644); err != nil {
		t.Fatalf("write .beads file: %v", err)
	}

	state, err := getGitState(repo)
	if err != nil {
		t.Fatalf("getGitState: %v", err)
	}

	if !state.Clean {
		t.Fatalf("expected worktree to be clean when only .beads changes exist, got %#v", state)
	}
	if len(state.UncommittedFiles) != 0 {
		t.Fatalf("expected no reported uncommitted files, got %v", state.UncommittedFiles)
	}
}

func TestGetGitState_ReportsNonBeadsChangesOnly(t *testing.T) {
	repo := initPolecatGitStateRepo(t)

	beadsFile := filepath.Join(repo, ".beads", "runtime.txt")
	if err := os.MkdirAll(filepath.Dir(beadsFile), 0755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	if err := os.WriteFile(beadsFile, []byte("runtime"), 0644); err != nil {
		t.Fatalf("write .beads file: %v", err)
	}

	realFile := filepath.Join(repo, "real.txt")
	if err := os.WriteFile(realFile, []byte("user work"), 0644); err != nil {
		t.Fatalf("write real file: %v", err)
	}

	state, err := getGitState(repo)
	if err != nil {
		t.Fatalf("getGitState: %v", err)
	}

	if state.Clean {
		t.Fatalf("expected worktree to be dirty when non-.beads changes exist, got %#v", state)
	}
	if len(state.UncommittedFiles) != 1 || state.UncommittedFiles[0] != "real.txt" {
		t.Fatalf("expected only real.txt to be reported, got %v", state.UncommittedFiles)
	}
}

func initPolecatGitStateRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")

	readme := filepath.Join(repo, "README.md")
	if err := os.WriteFile(readme, []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "init")
	return repo
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
