# Swarm Shutdown Design

Design for graceful swarm shutdown, worker cleanup, and session cycling.

**Epic**: gt-82y (Design: Swarm shutdown and worker cleanup)

## Key Decisions (from ultrathink)

1. **Pre-kill verification uses model intelligence** - Witness assesses git status output, not framework rules
2. **Witness can request restart** - Mail self handoff notes, exit cleanly when context filling
3. **Mayor NOT involved in per-worker cleanup** - That's Witness's domain
4. **Polecats verify themselves first** - Decommission checklist in prompting, Witness double-checks

## Responsibility Boundaries (gt-gl2)

### Mayor Responsibilities
- Swarm dispatch and strategic planning
- Cross-rig coordination
- Escalation handling (when Witness reports blocked workers)
- Final integration decisions
- **NOT**: Per-worker cleanup, session killing, nudging

### Witness Responsibilities
- Monitor worker health and progress
- Nudge workers toward completion
- Pre-kill verification (capture & assess git status)
- Session lifecycle (kill, restart workers)
- Self session cycling (mail handoff, exit)
- Report blocked workers to Mayor for escalation
- **NOT**: Implementation work, cross-rig coordination

### Polecat Responsibilities
- Complete assigned work
- Self-verify before signaling done (decommission checklist)
- Respond to Witness nudges
- **NOT**: Killing own session, coordinating with other polecats directly

## Subtask Designs

---

### gt-sd6: Enhanced Polecat Decommission Prompting

Add to polecat CLAUDE.md template (AGENTS.md.template):

```markdown
## Decommission Checklist

**CRITICAL**: Before signaling you are done, you MUST complete this checklist.
The Witness will verify each item and bounce you back if anything is dirty.

### Pre-Done Verification

Run these commands and verify ALL are clean:

```bash
# 1. Git status - must be clean (no uncommitted changes)
git status
# Expected: "nothing to commit, working tree clean"

# 2. Stash list - must be empty (no forgotten stashes)
git stash list
# Expected: (empty output)

# 3. Beads sync - must be up to date
bd sync --status
# Expected: "Up to date" or "Nothing to sync"

# 4. Branch merged - your work must be on main
git log main --oneline -1
git log HEAD --oneline -1
# Expected: Same commit (your branch is merged)
```

### If Any Check Fails

- **Uncommitted changes**: Commit them or discard if truly unnecessary
- **Stashes**: Pop and commit, or drop if obsolete
- **Beads out of sync**: Run `bd sync`
- **Branch not merged**: Complete the merge workflow

### Signaling Done

Only after ALL checks pass:

```bash
# Close your issue
bd close <issue-id>

# Final sync
bd sync

# Signal ready for decommission
town mail send <rig>/witness -s "Work Complete" -m "Issue <id> done. Checklist verified."
```

The Witness will capture your git state and verify before killing your session.
If anything is dirty, you'll receive a nudge with specific issues to fix.
```

---

### gt-f8v: Witness Pre-Kill Verification Protocol

Add to Witness CLAUDE.md template:

```markdown
## Pre-Kill Verification Protocol

Before killing any worker session, you MUST verify their workspace is clean.
Use your judgment on the output - don't rely on pattern matching.

### Verification Steps

When a worker signals done:

1. **Capture worker state**:
```bash
# Attach and capture git status
town capture <polecat> "git status && git stash list && git log --oneline -3"
```

2. **Assess the output** (use your judgment):
- Is working tree clean? (no modified/untracked files that matter)
- Is stash list empty? (or only contains intentional stashes)
- Does recent history show their work is committed?

3. **Decision**:
- **CLEAN**: Proceed to kill session
- **DIRTY**: Send nudge with specific issues

### Nudge Templates

**Uncommitted Changes**:
```
town inject <polecat> "WITNESS CHECK: You have uncommitted changes. Please commit or discard: <list files>. Signal done again when clean."
```

**Stash Not Empty**:
```
town inject <polecat> "WITNESS CHECK: You have stashed changes. Please pop and commit, or drop if obsolete: <stash list>. Signal done again when clean."
```

**Work Not Merged**:
```
town inject <polecat> "WITNESS CHECK: Your commits are not on main. Please complete merge workflow. Signal done again when merged."
```

**Multiple Issues**:
```
town inject <polecat> "WITNESS CHECK: Multiple issues found:
1. <issue 1>
2. <issue 2>
Please resolve all and signal done again."
```

### Kill Sequence

Only after verification passes:

```bash
# Log the verification
echo "[$(date)] Verified clean: <polecat>" >> witness/verification.log

# Kill the session
town kill <polecat>

# Update state
town sleep <polecat>
```

### Escalation

If a worker fails verification 3+ times or becomes unresponsive:

```bash
town mail send mayor/ -s "Escalation: <polecat> stuck" -m "Worker <polecat> cannot complete cleanup after 3 attempts. Issues: <list>. Requesting guidance."
```
```

---

### gt-eu9: Witness Session Cycling and Handoff

Add to Witness CLAUDE.md template:

```markdown
## Session Cycling

Your context will fill over long swarms. When you notice significant context usage
or feel you're losing track of state, proactively cycle your session.

### Recognizing When to Cycle

Signs you should cycle:
- You've been running for many hours
- You're losing track of which workers you've checked
- Responses are getting slower or less coherent
- You're about to start a complex operation

### Handoff Protocol

1. **Capture current state**:
```bash
# Check all worker states
town list .

# Check pending verifications
town all beads

# Check your inbox for unprocessed messages
town inbox
```

2. **Compose handoff note**:
```bash
town mail send <rig>/witness -s "Session Handoff" -m "$(cat <<'EOF'
[HANDOFF_TYPE]: witness_cycle
[TIMESTAMP]: $(date -Iseconds)
[RIG]: <rig>

## Active Workers
<list workers and their current status>

## Pending Verifications
<workers who signaled done but not yet verified>

## Recent Actions
<last 3-5 actions taken>

## Warnings/Notes
<anything the next session should know>

## Next Steps
<what should happen next>
EOF
)"
```

3. **Exit cleanly**:
```bash
# Ensure no pending operations
# Then simply end your session - the daemon will spawn a fresh one
```

### Handoff Note Format

The handoff note uses metadata format for parseability:

```
[HANDOFF_TYPE]: witness_cycle
[TIMESTAMP]: 2024-01-15T10:30:00Z
[RIG]: gastown

## Active Workers
- Furiosa: working on gt-abc1 (spawned 2h ago)
- Toast: idle, awaiting assignment
- Capable: signaled done, pending verification

## Pending Verifications
- Capable: signaled done at 10:25, not yet verified

## Recent Actions
1. Verified and killed Nux (gt-xyz9 complete)
2. Spawned Furiosa on gt-abc1
3. Received done signal from Capable

## Warnings/Notes
- Furiosa has been quiet for 30min, may need nudge
- Integration branch has 3 merged PRs

## Next Steps
1. Verify Capable's workspace
2. Check on Furiosa's progress
3. Report status to Refinery if all workers done
```

### On Fresh Session Start

When you start (or restart after cycling):

1. **Check for handoff**:
```bash
town inbox | grep "Session Handoff"
```

2. **If handoff exists, read it**:
```bash
town read <handoff-msg-id>
```

3. **Resume from handoff state** - pick up pending verifications, check noted workers

4. **If no handoff** - do full status check:
```bash
town list .
town all beads
```
```

---

### gt-gl2: Mayor vs Witness Cleanup Documentation

This goes in the main Gas Town documentation or CLAUDE.md templates.

```markdown
## Cleanup Authority Model

Gas Town uses a clear separation of cleanup responsibilities:

### The Rule
**Witness handles ALL per-worker cleanup. Mayor is never involved.**

### Why This Matters

1. **Separation of concerns**: Mayor thinks strategically, Witness thinks operationally
2. **Reduced coordination overhead**: No back-and-forth for routine cleanup
3. **Faster shutdown**: Witness can kill workers immediately upon verification
4. **Cleaner escalation**: Mayor only hears about problems, not routine operations

### What "Cleanup" Means

Witness handles:
- Verifying worker git state before kill
- Nudging workers to fix dirty state
- Killing worker sessions
- Updating worker state (sleep/wake)
- Logging verification results

Mayor handles:
- Receiving "swarm complete" notifications
- Deciding whether to start new swarms
- Handling escalations (stuck workers after multiple retries)
- Cross-rig coordination if workers need to hand off

### Escalation Path

```
Worker stuck -> Witness nudges (up to 3x) -> Witness escalates to Mayor
                                          -> Mayor decides: force kill, reassign, or human intervention
```

### Anti-Patterns

**DON'T**: Have Mayor ask Witness "is worker X clean?"
**DO**: Have Witness report "swarm complete, all workers verified and killed"

**DON'T**: Have Mayor kill worker sessions directly
**DO**: Have Mayor tell Witness "abort swarm" and let Witness handle cleanup

**DON'T**: Have workers report done to Mayor
**DO**: Have workers report done to Witness, Witness aggregates and reports to Refinery/Mayor
```

---

## Mail Templates (additions to templates.py)

### WORKER_DONE (Worker -> Witness)

```python
def worker_done(
    sender: str,
    rig: str,
    issue_id: str,
    checklist_verified: bool = True,
) -> Message:
    """Worker signals completion to Witness."""
    metadata = {
        "template": "WORKER_DONE",
        "rig": rig,
        "issue": issue_id,
        "checklist_verified": checklist_verified,
    }

    body = f"""Work complete on {issue_id}.

{_format_metadata(metadata)}

Decommission checklist {'verified' if checklist_verified else 'NOT verified - please check'}.
Ready for verification and session termination.
"""
    return Message.create(
        sender=sender,
        recipient=f"{rig}/witness",
        subject=f"Work Complete: {issue_id}",
        body=body,
    )
```

### VERIFICATION_FAILED (Witness -> Worker, via inject)

```python
def verification_failed(
    worker: str,
    issues: List[str],
) -> str:
    """Generate nudge text for failed verification (injected, not mailed)."""
    issues_text = "\n".join(f"  - {issue}" for issue in issues)
    return f"""WITNESS VERIFICATION FAILED

The following issues must be resolved before decommission:
{issues_text}

Please fix these issues and signal done again.
"""
```

### WITNESS_HANDOFF (Witness -> Witness)

```python
def witness_handoff(
    sender: str,
    rig: str,
    active_workers: List[Dict],
    pending_verifications: List[str],
    recent_actions: List[str],
    warnings: Optional[str] = None,
    next_steps: List[str] = None,
) -> Message:
    """Witness session handoff note."""
    metadata = {
        "template": "WITNESS_HANDOFF",
        "rig": rig,
        "timestamp": datetime.utcnow().isoformat(),
        "active_worker_count": len(active_workers),
        "pending_verification_count": len(pending_verifications),
    }

    # Format workers
    workers_text = "\n".join(
        f"- {w['name']}: {w['status']}" for w in active_workers
    ) or "None"

    # Format pending
    pending_text = "\n".join(f"- {p}" for p in pending_verifications) or "None"

    # Format actions
    actions_text = "\n".join(f"{i+1}. {a}" for i, a in enumerate(recent_actions[-5:]))

    body = f"""Session handoff for {rig} Witness.

{_format_metadata(metadata)}

## Active Workers
{workers_text}

## Pending Verifications
{pending_text}

## Recent Actions
{actions_text}

## Warnings
{warnings or "None"}

## Next Steps
{chr(10).join(f"- {s}" for s in (next_steps or ["Check pending verifications"]))}
"""
    return Message.create(
        sender=sender,
        recipient=f"{rig}/witness",
        subject="Session Handoff",
        body=body,
    )
```

### ESCALATION (Witness -> Mayor)

```python
def worker_escalation(
    sender: str,
    rig: str,
    worker: str,
    issue_id: str,
    attempts: int,
    unresolved_issues: List[str],
) -> Message:
    """Witness escalates stuck worker to Mayor."""
    metadata = {
        "template": "WORKER_ESCALATION",
        "rig": rig,
        "worker": worker,
        "issue": issue_id,
        "verification_attempts": attempts,
    }

    issues_text = "\n".join(f"  - {i}" for i in unresolved_issues)

    body = f"""Worker {worker} cannot complete cleanup.

{_format_metadata(metadata)}

After {attempts} verification attempts, the following issues remain:
{issues_text}

Requesting guidance:
1. Force kill and abandon changes?
2. Reassign to another worker?
3. Escalate to human?
"""
    return Message.create(
        sender=sender,
        recipient="mayor/",
        subject=f"Escalation: {worker} stuck on {issue_id}",
        body=body,
        priority="high",
    )
```

---

## Implementation Notes

### Verification State Tracking

Witness should track verification attempts in memory (or state.json):

```json
{
  "pending_verifications": {
    "Furiosa": {
      "issue_id": "gt-abc1",
      "signaled_at": "2024-01-15T10:25:00Z",
      "attempts": 1,
      "last_issues": ["uncommitted changes in src/foo.py"]
    }
  }
}
```

### Nudge vs Mail

- **Nudge (inject)**: For immediate attention - verification failures, progress checks
- **Mail**: For async communication - handoffs, escalations, status reports

### Timeout Handling

If worker doesn't respond to nudge within reasonable time:
1. First: Re-nudge with more urgency
2. Second: Capture their session state for diagnostics
3. Third: Escalate to Mayor

---

## Checklist for Implementation

- [ ] Update AGENTS.md.template with decommission checklist (gt-sd6)
- [ ] Create WITNESS_CLAUDE.md template with verification protocol (gt-f8v)
- [ ] Add session cycling to Witness prompting (gt-eu9)
- [ ] Document cleanup authority in main docs (gt-gl2)
- [ ] Add mail templates to templates.py
- [ ] Add verification state to Witness state.json schema
