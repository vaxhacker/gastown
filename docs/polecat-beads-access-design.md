# Polecat Beads Write Access Design

Design for granting polecats direct beads write access.

**Epic**: gt-l3c (Design: Polecat Beads write access)

## Background

Originally, polecats were read-only for beads to prevent multi-agent conflicts.
With Beads v0.30.0's tombstone-based rearchitecture for deletions, we now have
solid multi-agent support even at high concurrent load.

## Benefits

1. **Simplifies architecture** - No need for mail-based issue filing proxy via Witness
2. **Empowers polecats** - Can file discovered work that's out of their purview
3. **Beads handles work-disavowal** - Workers can close issues they didn't start
4. **Faster feedback** - No round-trip through Witness for issue creation

## Complications

For OSS projects where you're not a maintainer:
- Can't commit to the project's `.beads/` directory
- Need to file beads in a separate repo
- Beads supports this via `--root` flag

## Subtask Designs

---

### gt-zx3: Per-Rig Beads Configuration

#### Config Location

Per-rig configuration lives in the rig's config:

**Option A: In rig state.json** (simpler)
```
<rig>/config.json  (or state.json)
```

**Option B: In town-level rigs.json** (centralized)
```
config/rigs.json
```

Recommend **Option A** - each rig owns its config, easier to manage.

#### Config Schema

```json
// <rig>/config.json
{
  "version": 1,
  "name": "wyvern",
  "git_url": "https://github.com/steveyegge/wyvern",

  "beads": {
    // Where polecats file beads
    // Options: "local" | "<path>" | "<git-url>"
    "repo": "local",

    // Override bd --root (optional, derived from repo if not set)
    "root": null,

    // Issue prefix for this rig (used by bd create)
    "prefix": "wyv"
  }
}
```

#### Repo Options

| Value | Meaning | Use Case |
|-------|---------|----------|
| `"local"` | Use project's `.beads/` | Own projects, full commit access |
| `"<path>"` | Use beads at path | OSS contributions, external beads |
| `"<git-url>"` | Clone and use repo | Team shared beads |

#### Examples

**Local project (default)**:
```json
{
  "beads": {
    "repo": "local",
    "prefix": "wyv"
  }
}
```

**OSS contribution** (can't commit to project):
```json
{
  "beads": {
    "repo": "/home/user/my-beads/react-contributions",
    "prefix": "react"
  }
}
```

**Team shared beads**:
```json
{
  "beads": {
    "repo": "https://github.com/myteam/shared-beads",
    "prefix": "team"
  }
}
```

#### Environment Variable Injection

When spawning polecats, Gas Town sets:

```bash
export BEADS_ROOT="<resolved-path>"
# Polecats use bd normally; it respects BEADS_ROOT
```

Or pass explicit flag in spawn:
```bash
# Gas Town wraps bd calls internally
bd --root "$BEADS_ROOT" create --title="..."
```

#### Resolution Logic

```go
func ResolveBeadsRoot(rigConfig *RigConfig, rigPath string) (string, error) {
    beads := rigConfig.Beads

    switch {
    case beads.Root != "":
        // Explicit root override
        return beads.Root, nil

    case beads.Repo == "local" || beads.Repo == "":
        // Use project's .beads/
        return filepath.Join(rigPath, ".beads"), nil

    case strings.HasPrefix(beads.Repo, "/") || strings.HasPrefix(beads.Repo, "~"):
        // Absolute path
        return expandPath(beads.Repo), nil

    case strings.Contains(beads.Repo, "://"):
        // Git URL - need to clone
        return cloneAndResolve(beads.Repo)

    default:
        // Relative path from rig
        return filepath.Join(rigPath, beads.Repo), nil
    }
}
```

---

### gt-e1y: Worker Prompting - Beads Write Access

Add to polecat CLAUDE.md template (AGENTS.md.template):

```markdown
## Beads Access

You have **full beads access** - you can create, update, and close issues.

### Quick Reference

```bash
# View available work
bd ready                    # Issues ready to work (no blockers)
bd list                     # All open issues
bd show <id>                # Issue details

# Create issues
bd create --title="Fix login bug" --type=bug --priority=2
bd create --title="Add dark mode" --type=feature

# Update issues
bd update <id> --status=in_progress    # Claim work
bd close <id>                          # Mark complete
bd close <id> --reason="Duplicate of <other>"

# Sync (required before merge!)
bd sync                     # Commit beads changes to git
bd sync --status            # Check if sync needed
```

### When to Create Issues

Create beads issues when you discover work that:
- Is outside your current task scope
- Would benefit from tracking
- Should be done by someone else (or future you)

**Good examples**:
```bash
# Found a bug while implementing feature
bd create --title="Race condition in auth middleware" --type=bug --priority=1

# Noticed missing documentation
bd create --title="Document API rate limits" --type=task --priority=3

# Tech debt worth tracking
bd create --title="Refactor legacy payment module" --type=task --priority=4
```

**Don't create issues for**:
- Tiny fixes you can do in 2 minutes (just do them)
- Vague "improvements" with no clear scope
- Work that's already tracked elsewhere

### Issue Lifecycle

```
┌─────────┐    ┌─────────────┐    ┌──────────┐
│  open   │───►│ in_progress │───►│  closed  │
└─────────┘    └─────────────┘    └──────────┘
     │                                   ▲
     └───────────────────────────────────┘
              (can close directly)
```

You can close issues without claiming them first.
Useful for quick fixes or discovered duplicates.

### Beads Sync Protocol

**CRITICAL**: Always sync beads before merging to main!

```bash
# Before your final merge
bd sync                    # Commits beads changes
git status                 # Should show .beads/ changes
git add .beads/
git commit -m "beads: sync"
# Then proceed with merge to main
```

If you forget to sync, your beads changes will be lost when your session ends.

### Your Beads Repo

Your beads are configured for this rig. You don't need to specify --root.
Just use `bd` commands normally.

To check where your beads go:
```bash
bd config show root
```
```

---

### gt-cjb: Witness Updates - Remove Issue Filing Proxy

Update Witness CLAUDE.md to remove proxy responsibilities:

**REMOVE from Witness prompting**:

```markdown
## Issue Filing Proxy (REMOVED)

The following is NO LONGER your responsibility:
- Processing polecat "file issue" mail requests
- Creating issues on behalf of polecats
- Forwarding issue creation requests

Polecats now have direct beads write access and file their own issues.
```

**KEEP in Witness prompting** (from swarm-shutdown-design.md):
- Monitoring polecat progress
- Nudge protocol
- Pre-kill verification
- Session lifecycle management

**UPDATE**: If Witness receives an old-style "please file issue" request:

```markdown
### Legacy Issue Filing Requests

If you receive a mail asking you to file an issue on a polecat's behalf:

1. **Respond with update**:
```bash
town inject <polecat> "UPDATE: You have direct beads access now. Use 'bd create --title=\"...\" --type=...' to file issues yourself."
```

2. **Don't file the issue yourself** - let the polecat learn the new workflow.
```

---

### gt-082: Worker Cleanup - Beads Sync on Shutdown

This integrates with swarm-shutdown-design.md decommission checklist.

**Update to decommission checklist** (addition to gt-sd6):

```markdown
## Decommission Checklist (Updated)

### Pre-Done Verification

```bash
# 1. Git status - must be clean
git status
# Expected: "nothing to commit, working tree clean"

# 2. Stash list - must be empty
git stash list
# Expected: (empty)

# 3. Beads sync - MUST be synced
bd sync --status
# Expected: "Up to date" or "Nothing to sync"
# If not: run 'bd sync' first!

# 4. Beads committed - verify in git
git status
# Expected: .beads/ should NOT show changes
# If it does: git add .beads/ && git commit -m "beads: sync"

# 5. Branch merged to main
git log main --oneline -1
git log HEAD --oneline -1
# Expected: Same commit
```

### Beads Edge Cases

**Uncommitted beads changes**:
```bash
bd sync           # Commits to .beads/
git add .beads/
git commit -m "beads: final sync"
```

**Beads sync conflict** (rare):
```bash
# If bd sync fails with conflict:
git fetch origin main
git checkout main -- .beads/
bd sync --force   # Re-apply your changes
git add .beads/
git commit -m "beads: resolve sync conflict"
```
```

**Update to Witness pre-kill verification** (addition to gt-f8v):

```markdown
### Beads-Specific Verification

When capturing worker state, also check beads:

```bash
town capture <polecat> "bd sync --status && git status .beads/"
```

**Check for**:
- `bd sync --status` shows "Up to date"
- `git status .beads/` shows no changes

**If beads not synced**:
```
town inject <polecat> "WITNESS CHECK: Beads not synced. Run 'bd sync' then 'git add .beads/ && git commit -m \"beads: sync\"'. Signal done again when complete."
```
```

---

## Config File Examples

### Rig with local beads (default)

```json
// gastown/config.json
{
  "version": 1,
  "name": "gastown",
  "git_url": "https://github.com/steveyegge/gastown",
  "beads": {
    "repo": "local",
    "prefix": "gt"
  }
}
```

### Rig contributing to OSS project

```json
// react/config.json
{
  "version": 1,
  "name": "react",
  "git_url": "https://github.com/facebook/react",
  "beads": {
    "repo": "/home/steve/my-beads/react",
    "prefix": "react"
  }
}
```

### Rig with team shared beads

```json
// internal-app/config.json
{
  "version": 1,
  "name": "internal-app",
  "git_url": "https://github.com/mycompany/internal-app",
  "beads": {
    "repo": "https://github.com/mycompany/team-beads",
    "prefix": "app"
  }
}
```

---

## Migration Notes

### For Existing Rigs

1. Add `beads` section to rig config.json
2. Default to `"repo": "local"` if not specified
3. Update polecat CLAUDE.md templates
4. Remove Witness proxy code

### Backwards Compatibility

- If `beads` section missing, assume `"repo": "local"`
- Old-style "file issue" mail requests get redirect nudge
- No breaking changes for polecats already using bd read commands

---

## Implementation Checklist

- [ ] Add beads config schema to rig config (gt-zx3)
- [ ] Update polecat CLAUDE.md template with bd write access (gt-e1y)
- [ ] Update Witness CLAUDE.md to remove proxy, add redirect (gt-cjb)
- [ ] Update decommission checklist with beads sync (gt-082)
- [ ] Update Witness verification to check beads sync (gt-082)
- [ ] Add BEADS_ROOT environment injection to spawn logic
