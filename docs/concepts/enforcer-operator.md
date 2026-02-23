# Enforcer Operator

The Enforcer is a dedicated incident-control operator path for high-chaos situations
where normal patrol cadence and ad hoc coordination are too slow.

## Command Path

Enforcer runs as a managed crew workspace (equivalent lifecycle: start/status/restart/stop):

```bash
# Start
gt enforcer start

# Status
gt enforcer status

# Restart with fresh session
gt enforcer restart

# Stop
gt enforcer stop
```

Optional targeting:

```bash
gt enforcer start --rig gastown --name enforcer --agent codex
```

## Trigger Criteria

Use Enforcer when at least one of these conditions is true:

- Multiple rigs show simultaneous backlog growth, stuck queues, or repeated restart loops.
- Merge throughput is degraded and refinery/witness interventions are not stabilizing within one patrol cycle.
- Critical incident requires strict sequencing, authority boundaries, and explicit handback.
- Operator workload exceeds normal MEOW orchestration and needs a single incident commander session.

## Authority Boundaries

Enforcer is authorized to:

- Start, stop, and restart operational agents through standard `gt` commands.
- Coordinate emergency queue flow and dispatch priorities.
- Issue nudges/escalations and request focused remediation tasks.
- Apply temporary incident controls that are reversible and documented.

Enforcer is not authorized to:

- Bypass normal safety constraints in code or tooling.
- Perform destructive data actions without explicit incident justification and logging.
- Replace long-term ownership of mayor/deacon/refinery; it is an incident mode, not a permanent governance role.

## Handback Rules

When incident pressure is reduced:

1. Record current state and decisions in a handoff message.
2. Transfer active work ownership back to standard roles (Mayor/Deacon/rig agents).
3. Remove temporary controls or document why they remain.
4. Confirm queue/agent health has returned to steady-state.
5. Stop or park the Enforcer session if no longer needed.

Suggested handoff subject:

```text
🤝 HANDOFF: Enforcer incident control complete
```

## Validation (2026-02-23)

Validated in `gastown/mayor/rig` with a disposable worker name:

```bash
gt enforcer start --rig gastown --name enforcer_ci --agent codex
gt enforcer status --rig gastown --name enforcer_ci
gt enforcer restart --rig gastown --name enforcer_ci --agent codex
gt enforcer stop --rig gastown --name enforcer_ci
gt enforcer status --rig gastown --name enforcer_ci
gt crew remove enforcer_ci --rig gastown
```

Observed result:
- `start` creates the crew workspace and tmux session.
- `status` reports running/stopped state accurately.
- `restart` replaces the active session cleanly.
- `stop` terminates the session without deleting the workspace.

## Deploy and Rollback

Deploy steps:

```bash
go test ./internal/cmd -run TestEnforcer -count=1
go run ./cmd/gt enforcer status --rig gastown --name enforcer
git add internal/cmd/enforcer.go docs/concepts/enforcer-operator.md docs/reference.md README.md
git commit -m "Add Enforcer operator lifecycle command and runbook"
git push
```

Rollback steps:

```bash
# Disable live incident session first (if running)
gt enforcer stop --rig gastown --name enforcer

# Revert deployment commit on main
git revert <enforcer-commit-sha>
git push
```
