// Package witness provides the polecat monitoring agent.
//
// The EventLoop is the event-driven patrol implementation. Instead of
// instantiating a mol-witness-patrol formula every cycle (creating ~9 wisps
// per cycle), the witness runs a single long-lived loop that:
//
//  1. Tails .events.jsonl for immediate reaction to state changes
//  2. Runs periodic full-discovery patrols as a fallback
//
// This is the event-triggered + polling hybrid model: fast reaction from
// events, resilience from periodic polling.
package witness

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/steveyegge/gastown/internal/events"
	"github.com/steveyegge/gastown/internal/mail"
	"github.com/steveyegge/gastown/internal/workspace"
)

// EventLoopConfig configures the event-driven patrol loop.
type EventLoopConfig struct {
	// WorkDir is the witness working directory (rig root or witness/rig/).
	WorkDir string

	// RigName is the name of the rig being monitored.
	RigName string

	// TownRoot is the Gas Town root directory.
	TownRoot string

	// FullPatrolInterval is how often a full discovery patrol runs.
	// Default: 5 minutes.
	FullPatrolInterval time.Duration

	// EventDebounce is how long to wait after an event before reacting,
	// to batch rapid-fire events (e.g., multiple polecats finishing).
	// Default: 2 seconds.
	EventDebounce time.Duration
}

// EventLoop is the event-driven witness patrol loop.
type EventLoop struct {
	config   EventLoopConfig
	consumer *FeedConsumer
	router   *mail.Router

	// Metrics
	eventsProcessed int
	patrolsRun      int
	lastPatrolAt    time.Time
	lastEventAt     time.Time
}

// NewEventLoop creates a new event-driven patrol loop.
func NewEventLoop(config EventLoopConfig) (*EventLoop, error) {
	if config.FullPatrolInterval == 0 {
		config.FullPatrolInterval = 5 * time.Minute
	}
	if config.EventDebounce == 0 {
		config.EventDebounce = 2 * time.Second
	}
	if config.TownRoot == "" {
		townRoot, err := workspace.Find(config.WorkDir)
		if err != nil || townRoot == "" {
			config.TownRoot = config.WorkDir
		} else {
			config.TownRoot = townRoot
		}
	}

	eventsPath := filepath.Join(config.TownRoot, events.EventsFile)
	consumer, err := NewFeedConsumer(eventsPath)
	if err != nil {
		return nil, fmt.Errorf("creating feed consumer: %w", err)
	}

	router := mail.NewRouterWithTownRoot(config.WorkDir, config.TownRoot)

	return &EventLoop{
		config:   config,
		consumer: consumer,
		router:   router,
	}, nil
}

// Run starts the event loop. It blocks until the context is canceled.
// The loop:
//  1. Waits for events OR a patrol timer tick
//  2. On event: debounce, then handle the batch
//  3. On timer: run a full discovery patrol
func (el *EventLoop) Run(ctx context.Context) error {
	defer el.consumer.Close()

	patrolTicker := time.NewTicker(el.config.FullPatrolInterval)
	defer patrolTicker.Stop()

	// Run an initial full patrol on startup
	el.runFullPatrol()

	// Log that the event loop is running
	_ = events.LogFeed(events.TypePatrolStarted, el.config.RigName+"/witness",
		events.PatrolPayload(el.config.RigName, 0, "event-driven patrol started"))

	var pendingEvents []FeedEvent
	var debounceTimer *time.Timer
	var debounceCh <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			// Process any remaining pending events before exit
			if len(pendingEvents) > 0 {
				el.handleEventBatch(pendingEvents)
			}
			return ctx.Err()

		case event, ok := <-el.consumer.Events():
			if !ok {
				// Feed consumer closed
				return fmt.Errorf("feed consumer closed unexpectedly")
			}

			if !IsWitnessRelevant(event.Type) {
				continue
			}

			// Skip events from our own rig's witness to avoid feedback loops
			if strings.HasSuffix(event.Actor, "/witness") &&
				strings.HasPrefix(event.Actor, el.config.RigName+"/") {
				continue
			}

			pendingEvents = append(pendingEvents, event)
			el.lastEventAt = event.Timestamp

			// Start or reset debounce timer
			if debounceTimer == nil {
				debounceTimer = time.NewTimer(el.config.EventDebounce)
				debounceCh = debounceTimer.C
			} else {
				if !debounceTimer.Stop() {
					select {
					case <-debounceTimer.C:
					default:
					}
				}
				debounceTimer.Reset(el.config.EventDebounce)
			}

		case <-debounceCh:
			// Debounce period elapsed — process the batch
			if len(pendingEvents) > 0 {
				el.handleEventBatch(pendingEvents)
				pendingEvents = nil
			}
			debounceTimer = nil
			debounceCh = nil

		case <-patrolTicker.C:
			// Periodic full patrol — catches anything events miss
			el.runFullPatrol()
		}
	}
}

// handleEventBatch processes a batch of debounced events.
func (el *EventLoop) handleEventBatch(batch []FeedEvent) {
	for _, event := range batch {
		el.handleEvent(event)
		el.eventsProcessed++
	}
}

// handleEvent reacts to a single feed event by dispatching to the
// appropriate handler.
func (el *EventLoop) handleEvent(event FeedEvent) {
	switch event.Type {
	case "done":
		el.handleDoneEvent(event)

	case "merged":
		el.handleMergedEvent(event)

	case "merge_failed":
		el.handleMergeFailedEvent(event)

	case "session_death":
		el.handleSessionDeathEvent(event)

	case "mass_death":
		// Mass death — run full patrol immediately for comprehensive check
		log.Printf("[witness] mass death detected, running full patrol")
		el.runFullPatrol()

	case "spawn":
		// New polecat spawned — informational, no action needed
		log.Printf("[witness] polecat spawned: %s", PayloadString(event.Payload, "polecat"))

	case "kill":
		// Polecat killed — check for orphaned beads
		el.handleKillEvent(event)

	case "mail":
		// Mail sent — check if it's a HELP or other protocol message for us
		el.handleMailEvent(event)
	}
}

// handleDoneEvent reacts to a "done" event (polecat completed work).
// This is the event-driven equivalent of processing POLECAT_DONE mail.
func (el *EventLoop) handleDoneEvent(event FeedEvent) {
	beadID := PayloadString(event.Payload, "bead")
	branch := PayloadString(event.Payload, "branch")

	// Extract polecat name from actor (e.g., "gastown/polecats/nux" -> "nux")
	polecatName := extractPolecatNameFromActor(event.Actor)
	if polecatName == "" {
		return
	}

	// Only handle events for our rig
	if !strings.HasPrefix(event.Actor, el.config.RigName+"/") {
		return
	}

	log.Printf("[witness] done event: polecat=%s bead=%s branch=%s", polecatName, beadID, branch)

	// Build a synthetic POLECAT_DONE mail message for the existing handler
	msg := &mail.Message{
		Subject:   fmt.Sprintf("POLECAT_DONE %s", polecatName),
		Body:      fmt.Sprintf("Exit: COMPLETED\nIssue: %s\nBranch: %s\n", beadID, branch),
		Timestamp: event.Timestamp,
	}

	result := HandlePolecatDone(el.config.WorkDir, el.config.RigName, msg, el.router)
	if result.Error != nil {
		log.Printf("[witness] error handling done for %s: %v", polecatName, result.Error)
	}
	if result.Handled {
		log.Printf("[witness] done handled: %s -> %s", polecatName, result.Action)
	}
}

// handleMergedEvent reacts to a "merged" event from the refinery.
func (el *EventLoop) handleMergedEvent(event FeedEvent) {
	worker := PayloadString(event.Payload, "worker")
	branch := PayloadString(event.Payload, "branch")

	if worker == "" {
		return
	}

	log.Printf("[witness] merged event: worker=%s branch=%s", worker, branch)

	msg := &mail.Message{
		Subject:   fmt.Sprintf("MERGED %s", worker),
		Body:      fmt.Sprintf("Branch: %s\nMerged-At: %s\n", branch, event.Timestamp.Format(time.RFC3339)),
		Timestamp: event.Timestamp,
	}

	result := HandleMerged(el.config.WorkDir, el.config.RigName, msg)
	if result.Error != nil {
		log.Printf("[witness] error handling merged for %s: %v", worker, result.Error)
	}
	if result.Handled {
		log.Printf("[witness] merged handled: %s -> %s", worker, result.Action)
	}
}

// handleMergeFailedEvent reacts to a "merge_failed" event.
func (el *EventLoop) handleMergeFailedEvent(event FeedEvent) {
	worker := PayloadString(event.Payload, "worker")
	reason := PayloadString(event.Payload, "reason")
	branch := PayloadString(event.Payload, "branch")

	if worker == "" {
		return
	}

	log.Printf("[witness] merge_failed event: worker=%s reason=%s", worker, reason)

	msg := &mail.Message{
		Subject:   fmt.Sprintf("MERGE_FAILED %s", worker),
		Body:      fmt.Sprintf("Branch: %s\nFailureType: %s\nError: %s\n", branch, reason, reason),
		Timestamp: event.Timestamp,
	}

	result := HandleMergeFailed(el.config.WorkDir, el.config.RigName, msg, el.router)
	if result.Error != nil {
		log.Printf("[witness] error handling merge_failed for %s: %v", worker, result.Error)
	}
}

// handleSessionDeathEvent reacts to a session death.
func (el *EventLoop) handleSessionDeathEvent(event FeedEvent) {
	agent := PayloadString(event.Payload, "agent")
	reason := PayloadString(event.Payload, "reason")

	if agent == "" || !strings.HasPrefix(agent, el.config.RigName+"/") {
		return
	}

	log.Printf("[witness] session death: agent=%s reason=%s", agent, reason)

	// Run zombie detection for this rig — the dead session may have left
	// orphaned state that needs cleanup.
	result := DetectZombiePolecats(el.config.WorkDir, el.config.RigName, el.router)
	if len(result.Zombies) > 0 {
		log.Printf("[witness] found %d zombies after session death", len(result.Zombies))
		for _, z := range result.Zombies {
			log.Printf("[witness]   zombie: %s state=%s action=%s", z.PolecatName, z.AgentState, z.Action)
		}
	}
}

// handleKillEvent reacts to a polecat being killed.
func (el *EventLoop) handleKillEvent(event FeedEvent) {
	target := PayloadString(event.Payload, "target")
	if target == "" || !strings.HasPrefix(target, el.config.RigName+"/") {
		return
	}

	log.Printf("[witness] kill event: target=%s", target)

	// Check for orphaned beads left behind by the killed polecat
	result := DetectOrphanedBeads(el.config.WorkDir, el.config.RigName, el.router)
	if len(result.Orphans) > 0 {
		log.Printf("[witness] found %d orphaned beads after kill", len(result.Orphans))
	}
}

// handleMailEvent checks if a mail event is relevant to the witness
// (e.g., HELP messages sent to witness).
func (el *EventLoop) handleMailEvent(event FeedEvent) {
	to := PayloadString(event.Payload, "to")
	subject := PayloadString(event.Payload, "subject")

	// Only care about mail addressed to this rig's witness
	witnessAddr := el.config.RigName + "/witness"
	if to != witnessAddr {
		return
	}

	log.Printf("[witness] mail event to witness: subject=%s", subject)

	// The witness agent (Claude) handles mail in its patrol steps.
	// The event loop doesn't need to process mail content directly —
	// it just ensures the witness reacts quickly when mail arrives.
	// The existing inbox-check logic in the patrol formula handles this.
}

// runFullPatrol runs a comprehensive discovery patrol.
// This is the fallback that catches anything events miss.
func (el *EventLoop) runFullPatrol() {
	el.patrolsRun++
	el.lastPatrolAt = time.Now()

	log.Printf("[witness] full patrol #%d starting", el.patrolsRun)

	_ = events.LogFeed(events.TypePatrolStarted, el.config.RigName+"/witness",
		events.PatrolPayload(el.config.RigName, 0, fmt.Sprintf("full patrol #%d", el.patrolsRun)))

	// 1. Zombie detection — find dead polecats with active state
	zombieResult := DetectZombiePolecats(el.config.WorkDir, el.config.RigName, el.router)
	if len(zombieResult.Zombies) > 0 {
		log.Printf("[witness] patrol: found %d zombies (checked %d)", len(zombieResult.Zombies), zombieResult.Checked)
		for _, z := range zombieResult.Zombies {
			log.Printf("[witness]   zombie: %s state=%s action=%s", z.PolecatName, z.AgentState, z.Action)
		}
	}

	// 2. Orphaned bead detection — find beads assigned to dead polecats
	orphanResult := DetectOrphanedBeads(el.config.WorkDir, el.config.RigName, el.router)
	if len(orphanResult.Orphans) > 0 {
		log.Printf("[witness] patrol: found %d orphaned beads", len(orphanResult.Orphans))
		for _, o := range orphanResult.Orphans {
			log.Printf("[witness]   orphan: bead=%s polecat=%s recovered=%v", o.BeadID, o.PolecatName, o.BeadRecovered)
		}
	}

	// 3. Orphaned molecule detection
	molResult := DetectOrphanedMolecules(el.config.WorkDir, el.config.RigName, el.router)
	if len(molResult.Orphans) > 0 {
		log.Printf("[witness] patrol: found %d orphaned molecules", len(molResult.Orphans))
	}

	// 4. Stalled polecat detection (bypass-permissions prompts, etc.)
	stalledResult := DetectStalledPolecats(el.config.WorkDir, el.config.RigName)
	if len(stalledResult.Stalled) > 0 {
		log.Printf("[witness] patrol: found %d stalled polecats", len(stalledResult.Stalled))
		for _, s := range stalledResult.Stalled {
			log.Printf("[witness]   stalled: %s type=%s action=%s", s.PolecatName, s.StallType, s.Action)
		}
	}

	_ = events.LogFeed(events.TypePatrolComplete, el.config.RigName+"/witness",
		events.PatrolPayload(el.config.RigName, zombieResult.Checked,
			fmt.Sprintf("patrol #%d: %d zombies, %d orphans, %d stalled",
				el.patrolsRun, len(zombieResult.Zombies), len(orphanResult.Orphans), len(stalledResult.Stalled))))

	log.Printf("[witness] full patrol #%d complete", el.patrolsRun)
}

// extractPolecatNameFromActor extracts the polecat name from an actor string.
// Examples:
//
//	"gastown/polecats/nux" -> "nux"
//	"gastown/witness" -> ""
//	"mayor" -> ""
func extractPolecatNameFromActor(actor string) string {
	parts := strings.Split(actor, "/")
	if len(parts) >= 3 && parts[1] == "polecats" {
		return parts[2]
	}
	return ""
}

// Stats returns current event loop statistics.
func (el *EventLoop) Stats() EventLoopStats {
	return EventLoopStats{
		EventsProcessed: el.eventsProcessed,
		PatrolsRun:      el.patrolsRun,
		LastPatrolAt:     el.lastPatrolAt,
		LastEventAt:      el.lastEventAt,
	}
}

// EventLoopStats contains runtime statistics for the event loop.
type EventLoopStats struct {
	EventsProcessed int       `json:"events_processed"`
	PatrolsRun      int       `json:"patrols_run"`
	LastPatrolAt    time.Time `json:"last_patrol_at"`
	LastEventAt     time.Time `json:"last_event_at"`
}
