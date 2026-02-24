# Librarian Operator Runbook

The Librarian is an **on-demand** rig-level docs and knowledge operator.

The Mayor starts it with a specific task. It executes, submits changes through
the refinery (never pushes to main directly), and exits when done.

Each rig can run its own librarian session:
- `bd-librarian`
- `cm-librarian`
- `gt-librarian`

## Lifecycle

```
Start → Prime → Execute Task → Submit (gt done) → Exit
```

1. Mayor starts librarian with `gt librarian start <rig> --task "..."`
2. Librarian runs `gt prime` to load context
3. Librarian checks task sources (env var, hook, mail)
4. Librarian executes the assigned task
5. Librarian commits and submits via `gt done`
6. Session exits

## Quick Start

```bash
# Start with a task (preferred)
gt librarian start <rig> --task "Update docs for new retry logic"

# Start without explicit task (will check hook/mail)
gt librarian start <rig>

# Restart with new task
gt librarian restart <rig> --task "Audit docs/ for drift"

# Check status
gt librarian status <rig>

# Attach interactively
gt librarian attach <rig>
```

## Task Delivery Methods

| Method | Command | When to use |
|--------|---------|-------------|
| `--task` flag | `gt librarian start <rig> --task "..."` | Primary method — clear, explicit |
| Hook | `gt sling <bead-id> <rig>` (librarian picks up) | When task is tracked as a bead |
| Mail | `gt mail send <rig>/librarian -s "Task" -m "..."` | Additional instructions to running session |
| Nudge | `gt nudge <rig>/librarian "message"` | Quick follow-up to running session |

## Refinery Watch Mode

For monitoring a big merge and keeping docs current:

```bash
# Start librarian in watch mode
gt librarian start <rig> --task "Monitor refinery for <topic> merge. Review landed changes for doc impact. Batch updates and submit."

# Check what it submitted
gt mq list

# Check what gaps it filed
bd list --labels=docs,librarian
```

## Expected Output

The librarian produces:
- **Doc commits** submitted to the refinery merge queue (never pushed to main)
- **Doc beads** filed for gaps it couldn't resolve in-session
- **Escalation mail** to the mayor if blocked
- **Handoff mail** if work is partially complete

## Identity Model

Librarian identity is rig-scoped:
- `GT_ROLE=<rig>/librarian`
- `GT_RIG=<rig>`
- `BD_ACTOR=<rig>/librarian`

Examples:
- `beads/librarian`
- `cmtestsuite/librarian`
- `gastown/librarian`

## Troubleshooting

If you see `cannot determine agent identity (role: librarian)`:

1. Verify you are on a `gt` binary that includes librarian support:
```bash
gt librarian --help
```
2. Verify environment in session:
```bash
echo "$GT_ROLE $GT_RIG $BD_ACTOR"
```
3. Run from the librarian workdir:
```bash
cd ~/gt/<rig>/librarian
gt prime
```
4. Use explicit target as fallback:
```bash
gt mol status <rig>/librarian
gt hook status <rig>/librarian
```

If the librarian exits immediately without doing work:
- Check that a task was provided (`--task`, hook, or mail)
- Check `gt mail inbox` from the mayor for escalation messages
- Try `gt librarian attach <rig>` to see session output before it exits
