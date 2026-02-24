package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/session"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/tmux"
	"github.com/steveyegge/gastown/internal/workspace"
)

var (
	librarianStatusJSON    bool
	librarianAgentOverride string
	librarianEnvOverrides  []string
)

var librarianCmd = &cobra.Command{
	Use:     "librarian",
	GroupID: GroupAgents,
	Short:   "Manage the Librarian (per-rig docs and knowledge operator)",
	RunE:    requireSubcommand,
	Long: `Manage the Librarian - the per-rig docs and knowledge operations specialist.

The Librarian focuses on:
  - Keeping docs aligned with implementation behavior
  - Creating beads for doc/knowledge gaps
  - Improving runbooks, references, and discoverability

Role shortcuts: "librarian" in mail/nudge addresses resolves to this rig's Librarian.`,
}

var librarianStartCmd = &cobra.Command{
	Use:   "start [rig]",
	Short: "Start the librarian",
	Long: `Start the Librarian for a rig.

If rig is not specified, infers it from the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLibrarianStart,
}

var librarianStopCmd = &cobra.Command{
	Use:   "stop [rig]",
	Short: "Stop the librarian",
	Long: `Stop a running Librarian.

If rig is not specified, infers it from the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLibrarianStop,
}

var librarianStatusCmd = &cobra.Command{
	Use:   "status [rig]",
	Short: "Show librarian status",
	Long: `Show the status of a rig's Librarian.

If rig is not specified, infers it from the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLibrarianStatus,
}

var librarianAttachCmd = &cobra.Command{
	Use:     "attach [rig]",
	Aliases: []string{"at"},
	Short:   "Attach to librarian session",
	Long: `Attach to the Librarian tmux session for a rig.

If the librarian is not running, this will start it first.
If rig is not specified, infers it from the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLibrarianAttach,
}

var librarianRestartCmd = &cobra.Command{
	Use:   "restart [rig]",
	Short: "Restart the librarian",
	Long: `Restart the Librarian for a rig.

If rig is not specified, infers it from the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLibrarianRestart,
}

type LibrarianStatusOutput struct {
	Running bool   `json:"running"`
	RigName string `json:"rig_name"`
	Session string `json:"session,omitempty"`
}

func init() {
	librarianStartCmd.Flags().StringVar(&librarianAgentOverride, "agent", "", "Agent alias to run the Librarian with (overrides town default)")
	librarianStartCmd.Flags().StringArrayVar(&librarianEnvOverrides, "env", nil, "Environment variable override (KEY=VALUE, can be repeated)")

	librarianRestartCmd.Flags().StringVar(&librarianAgentOverride, "agent", "", "Agent alias to run the Librarian with (overrides town default)")
	librarianRestartCmd.Flags().StringArrayVar(&librarianEnvOverrides, "env", nil, "Environment variable override (KEY=VALUE, can be repeated)")

	librarianStatusCmd.Flags().BoolVar(&librarianStatusJSON, "json", false, "Output as JSON")

	librarianCmd.AddCommand(librarianStartCmd)
	librarianCmd.AddCommand(librarianStopCmd)
	librarianCmd.AddCommand(librarianRestartCmd)
	librarianCmd.AddCommand(librarianStatusCmd)
	librarianCmd.AddCommand(librarianAttachCmd)

	rootCmd.AddCommand(librarianCmd)
}

func getLibrarianRig(rigName string) (*RigReference, string, error) {
	if rigName == "" {
		townRoot, err := workspace.FindFromCwdOrError()
		if err != nil {
			return nil, "", fmt.Errorf("not in a Gas Town workspace: %w", err)
		}
		rigName, err = inferRigFromCwd(townRoot)
		if err != nil {
			return nil, "", fmt.Errorf("could not determine rig: %w\nUsage: gt librarian <command> <rig>", err)
		}
	}

	ref, err := loadRigReference(rigName)
	if err != nil {
		return nil, "", err
	}
	return ref, rigName, nil
}

func loadRigReference(rigName string) (*RigReference, error) {
	townRoot, r, err := getRig(rigName)
	if err != nil {
		return nil, err
	}
	return &RigReference{TownRoot: townRoot, RigPath: r.Path, RigName: r.Name}, nil
}

type RigReference struct {
	TownRoot string
	RigPath  string
	RigName  string
}

func librarianSessionName(rigName string) string {
	return fmt.Sprintf("%s-librarian", session.PrefixFor(rigName))
}

func librarianWorkDir(ref *RigReference) string {
	return filepath.Join(ref.RigPath, "librarian")
}

func runLibrarianStart(cmd *cobra.Command, args []string) error {
	rigName := ""
	if len(args) > 0 {
		rigName = args[0]
	}

	ref, rigName, err := getLibrarianRig(rigName)
	if err != nil {
		return err
	}

	if err := checkRigNotParkedOrDocked(rigName); err != nil {
		return err
	}

	workDir := librarianWorkDir(ref)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("creating librarian dir: %w", err)
	}

	sessionID := librarianSessionName(rigName)
	t := tmux.NewTmux()

	running, _ := t.HasSession(sessionID)
	if running {
		if t.IsAgentAlive(sessionID) {
			fmt.Printf("%s Librarian is already running for %s\n", style.Dim.Render("⚠"), rigName)
			return nil
		}
		if err := t.KillSessionWithProcesses(sessionID); err != nil {
			return fmt.Errorf("killing stale librarian session: %w", err)
		}
	}

	extraEnv, err := parseKeyValueOverrides(librarianEnvOverrides)
	if err != nil {
		return err
	}

	theme := tmux.AssignTheme(rigName)
	_, err = session.StartSession(t, session.SessionConfig{
		SessionID:      sessionID,
		WorkDir:        workDir,
		Role:           "librarian",
		TownRoot:       ref.TownRoot,
		RigPath:        ref.RigPath,
		RigName:        rigName,
		AgentName:      "librarian",
		AgentOverride:  librarianAgentOverride,
		ExtraEnv:       extraEnv,
		Theme:          &theme,
		WaitForAgent:   true,
		WaitFatal:      true,
		AcceptBypass:   true,
		ReadyDelay:     true,
		AutoRespawn:    true,
		TrackPID:       true,
		VerifySurvived: true,
	})
	if err != nil {
		return fmt.Errorf("starting librarian: %w", err)
	}

	fmt.Printf("%s Librarian started for %s\n", style.Bold.Render("✓"), rigName)
	fmt.Printf("  %s\n", style.Dim.Render("Use 'gt librarian attach' to connect"))
	fmt.Printf("  %s\n", style.Dim.Render("Use 'gt librarian status' to check status"))
	fmt.Printf("  %s\n", style.Dim.Render("In session, run 'gt prime' first"))
	return nil
}

func runLibrarianStop(cmd *cobra.Command, args []string) error {
	rigName := ""
	if len(args) > 0 {
		rigName = args[0]
	}

	_, rigName, err := getLibrarianRig(rigName)
	if err != nil {
		return err
	}

	sessionID := librarianSessionName(rigName)
	t := tmux.NewTmux()

	running, _ := t.HasSession(sessionID)
	if !running {
		fmt.Printf("%s Librarian is not running for %s\n", style.Dim.Render("⚠"), rigName)
		return nil
	}

	if err := t.KillSessionWithProcesses(sessionID); err != nil {
		return fmt.Errorf("stopping librarian: %w", err)
	}

	fmt.Printf("%s Librarian stopped for %s\n", style.Bold.Render("✓"), rigName)
	return nil
}

func runLibrarianStatus(cmd *cobra.Command, args []string) error {
	rigName := ""
	if len(args) > 0 {
		rigName = args[0]
	}

	_, rigName, err := getLibrarianRig(rigName)
	if err != nil {
		return err
	}

	sessionID := librarianSessionName(rigName)
	t := tmux.NewTmux()
	running, _ := t.HasSession(sessionID)

	if librarianStatusJSON {
		out := LibrarianStatusOutput{
			Running: running,
			RigName: rigName,
		}
		if running {
			out.Session = sessionID
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	fmt.Printf("%s Librarian: %s\n\n", style.Bold.Render(AgentTypeIcons[AgentWitness]), rigName)
	if running {
		fmt.Printf("  State: %s\n", style.Bold.Render("● running"))
		fmt.Printf("  Session: %s\n", sessionID)
	} else {
		fmt.Printf("  State: %s\n", style.Dim.Render("○ stopped"))
	}
	return nil
}

func runLibrarianAttach(cmd *cobra.Command, args []string) error {
	rigName := ""
	if len(args) > 0 {
		rigName = args[0]
	}

	_, rigName, err := getLibrarianRig(rigName)
	if err != nil {
		return err
	}

	sessionID := librarianSessionName(rigName)
	t := tmux.NewTmux()
	running, _ := t.HasSession(sessionID)
	if !running {
		if err := runLibrarianStart(cmd, []string{rigName}); err != nil {
			return err
		}
	}

	return attachToTmuxSession(sessionID)
}

func runLibrarianRestart(cmd *cobra.Command, args []string) error {
	rigName := ""
	if len(args) > 0 {
		rigName = args[0]
	}

	_, rigName, err := getLibrarianRig(rigName)
	if err != nil {
		return err
	}

	if err := checkRigNotParkedOrDocked(rigName); err != nil {
		return err
	}

	_ = runLibrarianStop(cmd, []string{rigName})
	return runLibrarianStart(cmd, []string{rigName})
}

func parseKeyValueOverrides(overrides []string) (map[string]string, error) {
	env := make(map[string]string, len(overrides))
	for _, override := range overrides {
		key, value, ok := strings.Cut(override, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("invalid --env override %q: expected KEY=VALUE", override)
		}
		env[key] = value
	}
	return env, nil
}
