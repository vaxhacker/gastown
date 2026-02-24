# Librarian Operator Runbook

The Librarian is a **rig-level** docs and knowledge operator.

Each rig can run its own librarian session:
- `bd-librarian`
- `cm-librarian`
- `gt-librarian`

## Quick Start

```bash
# Start (or restart) librarian for a rig
gt librarian start <rig>
gt librarian restart <rig>

# Check status
gt librarian status <rig>

# Attach interactively
gt librarian attach <rig>
```

Inside the librarian session, run:

```bash
gt prime
```

Then use normal operator flow:

```bash
gt mail inbox
gt mol status
gt hook status
bd ready
```

## Expected Behavior

Librarian is not a passive "wait for assignment" role. It should run a continuous docs-ops loop:

1. Read inbox + ready beads and pull active context.
2. Sample molecule/hook state to find undocumented workflows or failure modes.
3. Watch Refinery/Witness state for behavior changes that should be documented.
4. Create or update docs with concrete commands and verification steps.
5. Open/sling doc beads for remaining gaps in the owning rig.

## Identity Model

Librarian identity is rig-scoped:
- `GT_ROLE=<rig>/librarian`
- `GT_RIG=<rig>`
- `BD_ACTOR=<rig>/librarian`

Examples:
- `beads/librarian`
- `cmtestsuite/librarian`
- `gastown/librarian`

## Common Tasks

- Audit docs for drift against code.
- File doc debt and runbook gaps as beads in the owning rig.
- Route follow-up work with `gt sling`.
- Keep operator references actionable (real commands/paths).
- Randomly sample active/recent beads and molecule chains for hidden process gaps.
- Monitor Refinery changes and merge outcomes that need docs updates.

## High-Signal Commands

```bash
# Core loop inputs
gt mail inbox
bd ready
gt mol status
gt hook status
gt status

# Cross-check docs vs implementation
rg -n "term|command|path" docs internal README.md

# Open docs-focused follow-up work
bd create --type docs --title "Doc gap: <topic>"
gt sling <bead-id> <rig>
```

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
