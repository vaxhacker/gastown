package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/witness"
	"github.com/steveyegge/gastown/internal/workspace"
)

var (
	witnessPatrolInterval string
	witnessPatrolDebounce string
)

var witnessPatrolCmd = &cobra.Command{
	Use:   "patrol <rig>",
	Short: "Run event-driven witness patrol loop",
	Long: `Run the event-driven witness patrol loop for a rig.

This is the replacement for the mol-witness-patrol formula. Instead of
creating ~9 wisps per patrol cycle, the witness runs a single long-lived
event loop that:

  1. Tails ~/.events.jsonl for immediate reaction to state changes
     (POLECAT_DONE, MERGED, session deaths, etc.)
  2. Runs periodic full-discovery patrols as a fallback to catch
     anything events miss

The event loop handles:
  - done events -> auto-nuke clean polecats, cleanup wisps for dirty
  - merged events -> verify and nuke polecats
  - merge_failed events -> notify affected polecats
  - session_death events -> zombie detection
  - kill events -> orphaned bead detection
  - Periodic full patrol -> zombies, orphans, stalled agents

This is designed to run inside the witness tmux session. It blocks until
interrupted (SIGINT/SIGTERM) or the context is canceled.

Examples:
  gt witness patrol gastown
  gt witness patrol gastown --interval 3m
  gt witness patrol gastown --interval 10m --debounce 5s`,
	Args: cobra.ExactArgs(1),
	RunE: runWitnessPatrol,
}

func init() {
	witnessPatrolCmd.Flags().StringVar(&witnessPatrolInterval, "interval", "5m",
		"Full patrol interval (e.g., 3m, 5m, 10m)")
	witnessPatrolCmd.Flags().StringVar(&witnessPatrolDebounce, "debounce", "2s",
		"Event debounce window (e.g., 1s, 2s, 5s)")

	witnessCmd.AddCommand(witnessPatrolCmd)
}

func runWitnessPatrol(cmd *cobra.Command, args []string) error {
	rigName := args[0]

	// Resolve working directory
	_, r, err := getRig(rigName)
	if err != nil {
		return err
	}

	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	// Parse interval and debounce
	interval, err := time.ParseDuration(witnessPatrolInterval)
	if err != nil {
		return fmt.Errorf("invalid interval: %w", err)
	}

	debounce, err := time.ParseDuration(witnessPatrolDebounce)
	if err != nil {
		return fmt.Errorf("invalid debounce: %w", err)
	}

	// Determine witness working directory
	workDir := filepath.Join(r.Path, "witness", "rig")
	if _, statErr := os.Stat(workDir); os.IsNotExist(statErr) {
		workDir = filepath.Join(r.Path, "witness")
		if _, statErr := os.Stat(workDir); os.IsNotExist(statErr) {
			workDir = r.Path
		}
	}

	config := witness.EventLoopConfig{
		WorkDir:            workDir,
		RigName:            rigName,
		TownRoot:           townRoot,
		FullPatrolInterval: interval,
		EventDebounce:      debounce,
	}

	loop, err := witness.NewEventLoop(config)
	if err != nil {
		return fmt.Errorf("creating event loop: %w", err)
	}

	fmt.Printf("%s Witness event-driven patrol starting for %s\n", style.Bold.Render("●"), rigName)
	fmt.Printf("  %s Full patrol every %v, event debounce %v\n",
		style.Dim.Render("→"), interval, debounce)

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Printf("\n%s Shutting down witness patrol...\n", style.Dim.Render("⏹"))
		cancel()
	}()

	return loop.Run(ctx)
}
