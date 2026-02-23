package cmd

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/constants"
	"github.com/steveyegge/gastown/internal/crew"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

var (
	enforcerRig     string
	enforcerName    string
	enforcerAgent   string
	enforcerAccount string
)

var enforcerCmd = &cobra.Command{
	Use:     "enforcer",
	GroupID: GroupAgents,
	Short:   "Operate the Enforcer incident-control operator",
	Long: `Manage a dedicated Enforcer operator session for high-chaos incident control.

This command provides an opinionated operator entrypoint backed by a managed crew
workspace (equivalent lifecycle: start/status/restart/stop).`,
}

var enforcerStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Enforcer operator session",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runEnforcerStart(false)
	},
}

var enforcerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Enforcer operator status",
	RunE:  runEnforcerStatus,
}

var enforcerRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the Enforcer operator session",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runEnforcerStart(true)
	},
}

var enforcerStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Enforcer operator session",
	RunE:  runEnforcerStop,
}

func init() {
	enforcerCmd.PersistentFlags().StringVar(&enforcerRig, "rig", "gastown", "Rig hosting the Enforcer workspace")
	enforcerCmd.PersistentFlags().StringVar(&enforcerName, "name", "enforcer", "Crew worker name to use for Enforcer")
	enforcerCmd.PersistentFlags().StringVar(&enforcerAgent, "agent", "", "Runtime agent override (e.g., codex, claude, gemini)")
	enforcerCmd.PersistentFlags().StringVar(&enforcerAccount, "account", "", "Runtime account handle to use")

	enforcerCmd.AddCommand(enforcerStartCmd)
	enforcerCmd.AddCommand(enforcerStatusCmd)
	enforcerCmd.AddCommand(enforcerRestartCmd)
	enforcerCmd.AddCommand(enforcerStopCmd)
	rootCmd.AddCommand(enforcerCmd)
}

func runEnforcerStart(killExisting bool) error {
	crewMgr, r, err := getCrewManager(enforcerRig)
	if err != nil {
		return err
	}

	townRoot, _ := workspace.Find(r.Path)
	if townRoot == "" {
		townRoot = filepath.Dir(r.Path)
	}
	accountsPath := constants.MayorAccountsPath(townRoot)
	claudeConfigDir, _, _ := config.ResolveAccountConfigDir(accountsPath, enforcerAccount)

	opts := crew.StartOptions{
		Account:         enforcerAccount,
		ClaudeConfigDir: claudeConfigDir,
		KillExisting:    killExisting,
		Topic:           "enforcer",
		AgentOverride:   enforcerAgent,
	}

	if err := crewMgr.Start(enforcerName, opts); err != nil {
		if errors.Is(err, crew.ErrSessionRunning) && !killExisting {
			fmt.Printf("%s Enforcer already running\n", style.Dim.Render("○"))
			return runEnforcerStatus(nil, nil)
		}
		return err
	}

	sessionID := crewSessionName(r.Name, enforcerName)
	fmt.Printf("%s Enforcer %s/%s started\n", style.Bold.Render("✓"), r.Name, enforcerName)
	fmt.Printf("  Session: %s\n", sessionID)
	fmt.Printf("  Attach:  gt crew at %s/%s\n", r.Name, enforcerName)
	fmt.Printf("  Prime:   gt nudge %s/crew/%s \"Run gt prime and begin incident-control posture\"\n", r.Name, enforcerName)
	return nil
}

func runEnforcerStatus(cmd *cobra.Command, args []string) error {
	crewMgr, r, err := getCrewManager(enforcerRig)
	if err != nil {
		return err
	}

	worker, err := crewMgr.Get(enforcerName)
	if err != nil {
		if errors.Is(err, crew.ErrCrewNotFound) {
			fmt.Printf("%s Enforcer workspace %s/%s not created yet\n", style.Dim.Render("○"), r.Name, enforcerName)
			fmt.Printf("  Start with: gt enforcer start --rig %s --name %s\n", r.Name, enforcerName)
			return nil
		}
		return err
	}

	running, err := crewMgr.IsRunning(enforcerName)
	if err != nil {
		return err
	}

	state := "stopped"
	icon := style.Dim.Render("○")
	if running {
		state = "running"
		icon = style.Bold.Render("✓")
	}

	fmt.Printf("%s Enforcer status: %s\n", icon, state)
	fmt.Printf("  Rig:       %s\n", r.Name)
	fmt.Printf("  Worker:    %s\n", enforcerName)
	fmt.Printf("  Workspace: %s\n", worker.ClonePath)
	fmt.Printf("  Session:   %s\n", crewSessionName(r.Name, enforcerName))
	return nil
}

func runEnforcerStop(cmd *cobra.Command, args []string) error {
	crewMgr, r, err := getCrewManager(enforcerRig)
	if err != nil {
		return err
	}

	if err := crewMgr.Stop(enforcerName); err != nil {
		if errors.Is(err, crew.ErrSessionNotFound) {
			fmt.Printf("%s Enforcer not running\n", style.Dim.Render("○"))
			return nil
		}
		return err
	}

	fmt.Printf("%s Enforcer %s/%s stopped\n", style.Bold.Render("✓"), r.Name, enforcerName)
	return nil
}
