# Mayor Session Cycling and Handoff Design

Design for Mayor session management, context cycling, and structured handoff.

**Epic**: gt-u82 (Design: Mayor session cycling and handoff)

## Overview

Mayor coordinates across all rigs and runs for extended periods. Like Witness,
Mayor needs to cycle sessions when context fills, producing structured handoff
notes for the next session.

## Key Differences from Witness

| Aspect | Witness | Mayor |
|--------|---------|-------|
| Scope | Single rig, workers | All rigs, refineries |
| State tracking | Worker status, pending verifications | Active swarms, rig status, escalations |
| Handoff recipient | Self (same rig Witness) | Self (Mayor) |
| Complexity | Medium | Higher (cross-rig coordination) |
| Daemon | Witness daemon respawns | No daemon (manual or cron restart) |

## Design Areas

1. **Session cycling recognition** - When Mayor should cycle
2. **Handoff note format** - Structured state capture
3. **Handoff delivery** - Mail to self
4. **Fresh session startup** - Reading and resuming from handoff
5. **Integration with town commands** - CLI support

---

## 1. Session Cycling Recognition

### When to Cycle

Mayor should cycle when:
- Context is noticeably filling (responses slowing, losing track of state)
- Major phase completed (swarm finished, integration done)
- User requests session end
- Extended idle period with no active work

### Proactive vs Reactive

**Proactive** (preferred):
- Mayor notices context filling and initiates handoff
- Clean state capture while still coherent

**Reactive** (fallback):
- Session times out or crashes
- Less clean, may lose state

### Recognition Cues (for prompting)

```markdown
## Session Cycling

Monitor your context usage throughout the session. Signs you should cycle:

- You've been running for several hours
- You're having trouble remembering earlier conversation context
- You've completed a major phase of work
- Responses are taking longer than usual
- You're about to start a complex new operation

When you notice these signs, proactively initiate handoff rather than
waiting for problems.
```

---

## 2. Handoff Note Format

### Structure

Mayor handoff captures cross-rig state:

```
[HANDOFF_TYPE]: mayor_cycle
[TIMESTAMP]: 2024-01-15T14:30:00Z
[SESSION_DURATION]: 3h 45m

## Active Swarms
<per-rig swarm status>

## Rig Status
<health/state of each rig>

## Pending Escalations
<issues awaiting Mayor decision>

## In-Flight Decisions
<decisions being made, context needed>

## Recent Actions
<last 5-10 significant actions>

## Delegated Work
<work sent to refineries, awaiting response>

## User Requests
<any pending user requests>

## Next Steps
<what the next session should do>

## Warnings/Notes
<anything critical for next session>
```

### Example Handoff Note

```markdown
[HANDOFF_TYPE]: mayor_cycle
[TIMESTAMP]: 2024-01-15T14:30:00Z
[SESSION_DURATION]: 3h 45m

## Active Swarms

### gastown
- Status: Active swarm on auth feature
- Refinery: gastown/refinery coordinating
- Workers: 3 active (Furiosa, Toast, Capable)
- Issues: gt-auth-1, gt-auth-2, gt-auth-3
- Expected completion: Soon (2/3 issues merged)

### beads
- Status: Idle, no active swarm
- Last activity: 2h ago (maintenance work)

## Rig Status

| Rig | Health | Last Contact | Notes |
|-----|--------|--------------|-------|
| gastown | Good | 5min ago | Swarm active |
| beads | Good | 2h ago | Idle |

## Pending Escalations

1. **gastown/Toast stuck** - Witness escalated at 14:15
   - Issue: gt-auth-2 has merge conflict
   - Awaiting decision: reassign or manual fix?
   - Context: Toast tried 3 times, conflict in auth/middleware.go

## In-Flight Decisions

None currently.

## Recent Actions

1. 14:25 - Checked gastown swarm status
2. 14:20 - Received escalation re: Toast
3. 14:00 - Sent status request to beads/refinery
4. 13:30 - Dispatched auth swarm to gastown
5. 13:00 - Session started, read previous handoff

## Delegated Work

- gastown/refinery: Auth feature swarm (dispatched 13:30)
  - Expecting completion report when done

## User Requests

- User asked for auth feature implementation (completed dispatch)
- No other pending requests

## Next Steps

1. **Resolve Toast escalation** - Decide on reassign vs manual fix
2. **Monitor gastown swarm** - Should complete soon
3. **Check beads rig** - Been quiet, verify health

## Warnings/Notes

- Toast merge conflict is blocking swarm completion
- Consider waking another polecat if reassignment needed
```

---

## 3. Handoff Delivery

### Mail to Self

Mayor mails handoff to own inbox:

```bash
town mail send mayor/ -s "Session Handoff" -m "<handoff-content>"
```

### Why Mail (not file)?

- Consistent with Witness pattern
- Timestamped and logged
- Works across potential Mayor instances
- Integrates with existing inbox check on startup

### Handoff Template Function

```python
def mayor_handoff(
    active_swarms: List[SwarmStatus],
    rig_status: Dict[str, RigStatus],
    pending_escalations: List[Escalation],
    in_flight_decisions: List[Decision],
    recent_actions: List[str],
    delegated_work: List[DelegatedItem],
    user_requests: List[str],
    next_steps: List[str],
    warnings: Optional[str] = None,
    session_duration: Optional[str] = None,
) -> Message:
    """Create Mayor session handoff note."""

    metadata = {
        "template": "MAYOR_HANDOFF",
        "timestamp": datetime.utcnow().isoformat(),
        "session_duration": session_duration,
        "active_swarm_count": len(active_swarms),
        "pending_escalation_count": len(pending_escalations),
    }

    # ... format sections ...

    return Message.create(
        sender="mayor/",
        recipient="mayor/",
        subject="Session Handoff",
        body=body,
        priority="high",  # Ensure it's seen
    )
```

---

## 4. Fresh Session Startup

### Startup Protocol

When Mayor session starts:

1. **Check for handoff**:
```bash
town inbox | grep "Session Handoff"
```

2. **If handoff exists**:
```bash
# Read most recent handoff
town read <latest-handoff-id>

# Resume from handoff state
# - Address pending escalations first
# - Check on in-flight work
# - Continue with next steps
```

3. **If no handoff** (fresh start):
```bash
# Full system status check
town status
town rigs
bd ready

# Check all rig inboxes for pending items
town inbox
```

### Handoff Processing

```markdown
## On Session Start

1. **Check inbox for handoff**:
```bash
town inbox
```
Look for "Session Handoff" messages.

2. **If handoff found**:
   - Read the handoff note
   - Process pending escalations (highest priority)
   - Check status of noted swarms
   - Verify rig health matches notes
   - Continue with documented next steps

3. **If no handoff**:
   - Do full status check: `town status`
   - Check each rig: `town rigs`
   - Check inbox for any messages
   - Check beads for work: `bd ready`

4. **After processing handoff**:
   - Archive or delete the handoff message
   - You now own the current state
```

---

## 5. Integration with Town Commands

### New Commands (optional, can be deferred)

```bash
# Generate handoff note interactively
town handoff

# Generate and send in one step
town handoff --send

# Check for handoff on startup
town resume
```

### Implementation

For now, Mayor does this manually in prompting. Later can add CLI support:

```go
// cmd/gt/cmd/handoff.go
var handoffCmd = &cobra.Command{
    Use:   "handoff",
    Short: "Generate session handoff note",
    Run: func(cmd *cobra.Command, args []string) {
        // Gather state
        swarms := gatherActiveSwarms()
        rigs := gatherRigStatus()
        // ... etc

        // Format handoff
        note := formatHandoffNote(swarms, rigs, ...)

        if send {
            // Send to mayor inbox
            mail.Send("mayor/", "Session Handoff", note)
        } else {
            // Print for review
            fmt.Println(note)
        }
    },
}
```

---

## Subtasks

Based on this design, create these implementation subtasks:

### gt-u82.1: Mayor session cycling prompting

Add to Mayor CLAUDE.md:
- When to cycle recognition
- How to compose handoff note
- Handoff format specification

### gt-u82.2: Mayor startup protocol prompting

Add to Mayor CLAUDE.md:
- Check for handoff on start
- Process handoff content
- Fresh start fallback

### gt-u82.3: Mayor handoff mail template

Add to templates.py:
- MAYOR_HANDOFF template
- Parsing utilities

### gt-u82.4: (Optional) town handoff command

CLI support for handoff generation:
- `town handoff` - generate interactively
- `town handoff --send` - generate and mail
- `town resume` - check for and display handoff

---

## Prompting Additions

### Mayor CLAUDE.md - Session Management Section

```markdown
## Session Management

### Recognizing When to Cycle

Monitor your session health. Cycle proactively when:
- You've been running for several hours
- Context feels crowded (losing track of earlier state)
- Major phase completed (good stopping point)
- About to start complex new work

Don't wait for problems - proactive handoff produces cleaner state.

### Creating Handoff Notes

Before ending your session, capture current state:

1. **Gather information**:
```bash
town status                 # Overall health
town rigs                   # Each rig's state
town inbox                  # Pending messages
bd ready                    # Work items
```

2. **Compose handoff note** with this structure:

```
[HANDOFF_TYPE]: mayor_cycle
[TIMESTAMP]: <current time>
[SESSION_DURATION]: <how long you've been running>

## Active Swarms
<list each rig with active swarm, workers, progress>

## Rig Status
<table of rig health>

## Pending Escalations
<issues needing your decision>

## In-Flight Decisions
<decisions you were making>

## Recent Actions
<last 5-10 things you did>

## Delegated Work
<work sent to refineries>

## User Requests
<any pending user asks>

## Next Steps
<what next session should do>

## Warnings/Notes
<critical info for next session>
```

3. **Send handoff**:
```bash
town mail send mayor/ -s "Session Handoff" -m "<your handoff note>"
```

4. **End session** - next instance will pick up from handoff.

### On Session Start

1. **Check for handoff**:
```bash
town inbox | grep "Session Handoff"
```

2. **If found, read it**:
```bash
town read <msg-id>
```

3. **Process in priority order**:
   - Pending escalations (urgent)
   - In-flight decisions (context-dependent)
   - Check noted swarm status (may have changed)
   - Continue with next steps

4. **If no handoff**:
```bash
town status
town rigs
bd ready
town inbox
```
   Build your own picture of current state.

### Handoff Best Practices

- **Be specific** - "Toast has merge conflict in auth/middleware.go" not "Toast is stuck"
- **Include context** - Why decisions are pending, what you were thinking
- **Prioritize next steps** - What's most urgent
- **Note time-sensitive items** - Anything that might have changed since handoff
```

---

## Implementation Checklist

- [ ] Create subtasks (gt-u82.1 through gt-u82.4)
- [ ] Add session management section to Mayor CLAUDE.md template
- [ ] Add MAYOR_HANDOFF template to templates.py
- [ ] Update startup instructions in Mayor prompting
- [ ] (Optional) Implement town handoff command
