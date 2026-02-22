package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/events"
	"github.com/steveyegge/gastown/internal/nudge"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/tmux"
	"github.com/steveyegge/gastown/internal/workspace"
)

var (
	wallRig    string
	wallDryRun bool
	wallMode   string
)

func init() {
	wallCmd.Flags().StringVar(&wallRig, "rig", "", "Only send to agents in this rig")
	wallCmd.Flags().BoolVar(&wallDryRun, "dry-run", false, "Show what would be sent without sending")
	wallCmd.Flags().StringVar(&wallMode, "mode", NudgeModeQueue, "Delivery mode: queue (default), immediate, or wait-idle")
	rootCmd.AddCommand(wallCmd)
}

var wallCmd = &cobra.Command{
	Use:     "wall <message>",
	GroupID: GroupComm,
	Short:   "Broadcast message to all active agents (like Unix wall)",
	Long: `Broadcast a message to ALL active agent sessions in town.

Like Unix wall(1), this sends a message to every running agent: polecats,
crew, witnesses, refineries, mayor, and deacon. Use this for town-wide
announcements like maintenance windows, priority changes, or coordination.

The message is delivered via nudge queue by default (non-disruptive).
Use --mode=immediate to interrupt agents directly.

The sender (self) is automatically excluded from delivery.

Examples:
  gt wall "System maintenance in 10 minutes"
  gt wall "Priority change: all agents check mail"
  gt wall --rig gastown "New work available in gastown"
  gt wall --mode=immediate "URGENT: stop all work"
  gt wall --dry-run "Test announcement"`,
	Args: cobra.ExactArgs(1),
	RunE: runWall,
}

func runWall(cmd *cobra.Command, args []string) error {
	message := args[0]

	if message == "" {
		return fmt.Errorf("message cannot be empty")
	}

	// Validate --mode
	if !validNudgeModes[wallMode] {
		return fmt.Errorf("invalid --mode %q: must be one of immediate, queue, wait-idle", wallMode)
	}

	// Get all agent sessions (including polecats)
	agents, err := getAgentSessions(true)
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}

	// Get sender identity to exclude self
	sender := os.Getenv("BD_ACTOR")
	if sender == "" {
		if roleInfo, err := GetRole(); err == nil {
			sender = formatRoleSender(roleInfo)
		}
	}

	// Filter to target agents
	var targets []*AgentSession
	for _, agent := range agents {
		// Filter by rig if specified
		if wallRig != "" && agent.Rig != wallRig {
			continue
		}

		// Skip self to avoid interrupting own session
		if sender != "" && formatAgentName(agent) == sender {
			continue
		}

		targets = append(targets, agent)
	}

	if len(targets) == 0 {
		fmt.Println("No agents running to broadcast to.")
		if wallRig != "" {
			fmt.Printf("  (filtered by rig: %s)\n", wallRig)
		}
		return nil
	}

	// Dry run - just show what would be sent
	if wallDryRun {
		fmt.Printf("Would broadcast to %d agent(s):\n\n", len(targets))
		for _, agent := range targets {
			fmt.Printf("  %s %s\n", AgentTypeIcons[agent.Type], formatAgentName(agent))
		}
		fmt.Printf("\nMessage: %s\n", message)
		fmt.Printf("Mode: %s\n", wallMode)
		return nil
	}

	townRoot, _ := workspace.FindFromCwd()
	t := tmux.NewTmux()
	var succeeded, failed, skipped int
	var failures []string

	fmt.Printf("Wall broadcast to %d agent(s) (mode=%s)...\n\n", len(targets), wallMode)

	for i, agent := range targets {
		agentName := formatAgentName(agent)

		// Check DND status before nudging
		if townRoot != "" {
			if shouldSend, level, _ := shouldNudgeTarget(townRoot, agentName, false); !shouldSend {
				skipped++
				fmt.Printf("  %s %s %s (DND: %s)\n", style.Dim.Render("○"), AgentTypeIcons[agent.Type], agentName, level)
				continue
			}
		}

		if err := wallDeliver(t, townRoot, agent.Name, message, sender); err != nil {
			failed++
			failures = append(failures, fmt.Sprintf("%s: %v", agentName, err))
			fmt.Printf("  %s %s %s\n", style.ErrorPrefix, AgentTypeIcons[agent.Type], agentName)
		} else {
			succeeded++
			fmt.Printf("  %s %s %s\n", style.SuccessPrefix, AgentTypeIcons[agent.Type], agentName)
		}

		// Small delay between nudges to avoid overwhelming tmux
		if i < len(targets)-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	fmt.Println()

	// Log feed event
	if sender != "" {
		rigTarget := ""
		if wallRig != "" {
			rigTarget = wallRig
		}
		_ = events.LogFeed(events.TypeNudge, sender, events.NudgePayload(rigTarget, "wall", message))
	}

	if failed > 0 {
		summary := fmt.Sprintf("Wall complete: %d delivered, %d failed", succeeded, failed)
		if skipped > 0 {
			summary += fmt.Sprintf(", %d skipped (DND)", skipped)
		}
		fmt.Printf("%s %s\n", style.WarningPrefix, summary)
		for _, f := range failures {
			fmt.Printf("  %s\n", style.Dim.Render(f))
		}
		return fmt.Errorf("%d delivery(s) failed", failed)
	}

	summary := fmt.Sprintf("Wall complete: %d agent(s) notified", succeeded)
	if skipped > 0 {
		summary += fmt.Sprintf(", %d skipped (DND)", skipped)
	}
	fmt.Printf("%s %s\n", style.SuccessPrefix, summary)
	return nil
}

// wallDeliver sends a wall message using the configured delivery mode.
func wallDeliver(t *tmux.Tmux, townRoot, sessionName, message, sender string) error {
	prefixedMessage := fmt.Sprintf("[wall from %s] %s", sender, message)

	switch wallMode {
	case NudgeModeQueue:
		if townRoot == "" {
			return fmt.Errorf("--mode=queue requires a Gas Town workspace")
		}
		return nudge.Enqueue(townRoot, sessionName, nudge.QueuedNudge{
			Sender:   sender,
			Message:  fmt.Sprintf("[wall] %s", message),
			Priority: nudge.PriorityNormal,
		})

	case NudgeModeWaitIdle:
		if townRoot == "" {
			return fmt.Errorf("--mode=wait-idle requires a Gas Town workspace")
		}
		err := t.WaitForIdle(sessionName, waitIdleTimeout)
		if err == nil {
			return t.NudgeSession(sessionName, prefixedMessage)
		}
		// Timeout — fall back to queue
		return nudge.Enqueue(townRoot, sessionName, nudge.QueuedNudge{
			Sender:   sender,
			Message:  fmt.Sprintf("[wall] %s", message),
			Priority: nudge.PriorityNormal,
		})

	default: // NudgeModeImmediate
		return t.NudgeSession(sessionName, prefixedMessage)
	}
}

// formatRoleSender formats a RoleInfo into a sender string.
func formatRoleSender(roleInfo RoleInfo) string {
	switch roleInfo.Role {
	case RoleMayor:
		return "mayor"
	case RoleCrew:
		return fmt.Sprintf("%s/crew/%s", roleInfo.Rig, roleInfo.Polecat)
	case RolePolecat:
		return fmt.Sprintf("%s/%s", roleInfo.Rig, roleInfo.Polecat)
	case RoleWitness:
		return fmt.Sprintf("%s/witness", roleInfo.Rig)
	case RoleRefinery:
		return fmt.Sprintf("%s/refinery", roleInfo.Rig)
	case RoleDeacon:
		return "deacon"
	default:
		return string(roleInfo.Role)
	}
}
