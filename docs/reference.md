# Gas Town Reference

Technical reference for Gas Town internals. Read the README first.

## Directory Structure

```
~/gt/                           Town root
├── .beads/                     Town-level beads (hq-* prefix)
├── CLAUDE.md                   Identity anchor (run gt prime)
├── mayor/                      Mayor agent home (town coordinator)
│   ├── town.json               Town configuration
│   └── .claude/settings.local.json  Mayor Claude settings
├── deacon/                     Deacon agent home (background supervisor)
│   └── .claude/settings.local.json  Deacon settings (context via gt prime)
└── <rig>/                      Project container (NOT a git clone)
    ├── config.json             Rig identity
    ├── .beads/ → mayor/rig/.beads
    ├── .repo.git/              Bare repo (shared by worktrees)
    ├── mayor/rig/              Mayor's clone (canonical beads, NOT an agent)
    ├── witness/                Witness agent home (monitors only)
    ├── refinery/               Refinery agent home
    │   └── rig/                Worktree on main
    ├── crew/                   Crew parent
    │   └── <name>/             Human workspaces (worktrees)
    └── polecats/               Polecats parent
        └── <name>/<rigname>/   Worker worktrees
```

**Key points:**

- Only `~/gt/CLAUDE.md` exists on disk — a minimal identity anchor
- No per-directory CLAUDE.md or AGENTS.md — full context comes from `gt prime` via SessionStart hook
- Settings use `settings.local.json` at each agent's working directory
- Rig root is a container, not a clone
- `.repo.git/` is bare — refinery and polecats are worktrees
- Per-rig `mayor/rig/` holds canonical `.beads/`, others inherit via redirect
- Per-rig `mayor/rig/` is just a source clone, NOT an agent directory

## Beads Routing

Gas Town routes beads commands based on issue ID prefix. You don't need to think
about which database to use - just use the issue ID.

```bash
bd show gp-xyz    # Routes to greenplace rig's beads
bd show hq-abc    # Routes to town-level beads
bd show wyv-123   # Routes to wyvern rig's beads
```

**How it works**: Routes are defined in `~/gt/.beads/routes.jsonl`. Each rig's
prefix maps to its beads location (the mayor's clone in that rig).

| Prefix | Routes To | Purpose |
|--------|-----------|---------|
| `hq-*` | `~/gt/.beads/` | Mayor mail, cross-rig coordination |
| `gp-*` | `~/gt/greenplace/mayor/rig/.beads/` | Greenplace project issues |
| `wyv-*` | `~/gt/wyvern/mayor/rig/.beads/` | Wyvern project issues |

Debug routing: `BD_DEBUG_ROUTING=1 bd show <id>`

## Configuration

### Rig Config (`config.json`)

```json
{
  "type": "rig",
  "name": "myproject",
  "git_url": "https://github.com/...",
  "default_branch": "main",
  "beads": { "prefix": "mp" }
}
```

**Rig config fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `git_url` | `string` | required | The git URL for the rig's repository. |
| `default_branch` | `string` | `"main"` | Default branch for the rig. Auto-detected from remote during `gt rig add`. Used as the merge target by the Refinery and as the base for polecats when no integration branch is active. |

### Git Remote Layout

> **CRITICAL**: Every clone, worktree, and rig repo MUST use this layout:

| Remote | Points to | Purpose |
|--------|-----------|---------|
| `origin` | **Your fork** (e.g. `vaxhacker/<repo>`) | Fetch AND push. This is YOUR canonical repo. |
| `upstream` | **Upstream** (e.g. `steveyegge/<repo>`) | Read-only. For syncing. **NEVER push here.** |

**Rules:**
- `origin` fetch URL and push URL MUST be the same repo (your fork).
- **NEVER** create split remotes (fetch from one repo, push to another).
- **NEVER** create extra remotes like `forktmp`, `pushfork`, or `fork`.
- If `origin` points to the wrong place, every `git pull`, `git push`, `make install`, and staleness check will break in confusing ways.

To sync from upstream: `git fetch upstream && git cherry-pick <sha> && git push origin main`

### Settings (`settings/config.json`)

```json
{
  "theme": "desert",
  "merge_queue": {
    "enabled": true,
    "run_tests": true,
    "setup_command": "",
    "typecheck_command": "",
    "lint_command": "",
    "test_command": "go test ./...",
    "build_command": "",
    "on_conflict": "assign_back",
    "delete_merged_branches": true,
    "retry_flaky_tests": 1,
    "poll_interval": "30s",
    "max_concurrent": 1,
    "integration_branch_polecat_enabled": true,
    "integration_branch_refinery_enabled": true,
    "integration_branch_template": "integration/{title}",
    "integration_branch_auto_land": false
  }
}
```

**Merge queue fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | `bool` | `true` | Whether the merge queue is active |
| `run_tests` | `bool` | `true` | Run tests before merging |
| `setup_command` | `string` | `""` | Setup/install command (e.g., `pnpm install`) |
| `typecheck_command` | `string` | `""` | Type check command (e.g., `tsc --noEmit`) |
| `lint_command` | `string` | `""` | Lint command (e.g., `eslint .`) |
| `test_command` | `string` | `"go test ./..."` | Test command to run |
| `build_command` | `string` | `""` | Build command (e.g., `go build ./...`) |
| `on_conflict` | `string` | `"assign_back"` | Conflict strategy: `assign_back` or `auto_rebase` |
| `delete_merged_branches` | `bool` | `true` | Delete source branches after merging |
| `retry_flaky_tests` | `int` | `1` | Number of times to retry flaky tests |
| `poll_interval` | `string` | `"30s"` | How often Refinery polls for new MRs |
| `max_concurrent` | `int` | `1` | Maximum concurrent merges |
| `integration_branch_polecat_enabled` | `*bool` | `true` | Polecats auto-source worktrees from integration branches |
| `integration_branch_refinery_enabled` | `*bool` | `true` | `gt done` / `gt mq submit` auto-target integration branches |
| `integration_branch_template` | `string` | `"integration/{title}"` | Branch name template (`{title}`, `{epic}`, `{prefix}`, `{user}`) |
| `integration_branch_auto_land` | `*bool` | `false` | Refinery patrol auto-lands when all children closed |

See [Integration Branches](concepts/integration-branches.md) for integration branch details.

### Runtime (`.runtime/` - gitignored)

Process state, PIDs, ephemeral data.

### Rig-Level Configuration

Rigs support layered configuration through:
1. **Wisp layer** (`.beads-wisp/config/`) - transient, local overrides
2. **Rig identity bead labels** - persistent rig settings
3. **Town defaults** (`~/gt/settings/config.json`)
4. **System defaults** - compiled-in fallbacks

#### Polecat Branch Naming

Configure custom branch name templates for polecats:

```bash
# Set via wisp (transient - for testing)
echo '{"polecat_branch_template": "adam/{year}/{month}/{description}"}' > \
  ~/gt/.beads-wisp/config/myrig.json

# Or set via rig identity bead labels (persistent)
bd update gt-rig-myrig --labels="polecat_branch_template:adam/{year}/{month}/{description}"
```

**Template Variables:**

| Variable | Description | Example |
|----------|-------------|---------|
| `{user}` | From `git config user.name` | `adam` |
| `{year}` | Current year (YY format) | `26` |
| `{month}` | Current month (MM format) | `01` |
| `{name}` | Polecat name | `alpha` |
| `{issue}` | Issue ID without prefix | `123` (from `gt-123`) |
| `{description}` | Sanitized issue title | `fix-auth-bug` |
| `{timestamp}` | Unique timestamp | `1ks7f9a` |

**Default Behavior (backward compatible):**

When `polecat_branch_template` is empty or not set:
- With issue: `polecat/{name}/{issue}@{timestamp}`
- Without issue: `polecat/{name}-{timestamp}`

**Example Configurations:**

```bash
# GitHub enterprise format
"adam/{year}/{month}/{description}"

# Simple feature branches
"feature/{issue}"

# Include polecat name for clarity
"work/{name}/{issue}"
```

## Formula Format

```toml
formula = "name"
type = "workflow"           # workflow | expansion | aspect
version = 1
description = "..."

[vars.feature]
description = "..."
required = true

[[steps]]
id = "step-id"
title = "{{feature}}"
description = "..."
needs = ["other-step"]      # Dependencies
```

**Composition:**

```toml
extends = ["base-formula"]

[compose]
aspects = ["cross-cutting"]

[[compose.expand]]
target = "step-id"
with = "macro-formula"
```

## Molecule Lifecycle

```
Formula (source TOML) ─── "Ice-9"
    │
    ▼ bd cook
Protomolecule (frozen template) ─── Solid
    │
    ├─▶ bd mol pour ──▶ Mol (persistent) ─── Liquid ──▶ bd squash ──▶ Digest
    │
    └─▶ bd mol wisp ──▶ Wisp (ephemeral) ─── Vapor ──┬▶ bd squash ──▶ Digest
                                                  └▶ bd burn ──▶ (gone)
```

**Note**: Wisps are stored in `.beads/` with an ephemeral flag - they're not
persisted to JSONL. They exist only in memory during execution.

## Molecule Commands

**Principle**: `bd` = beads data operations, `gt` = agent operations.

### Beads Operations (bd)

```bash
# Formulas
bd formula list              # Available formulas
bd formula show <name>       # Formula details
bd cook <formula>            # Formula → Proto

# Molecules (data operations)
bd mol list                  # Available protos
bd mol show <id>             # Proto details
bd mol pour <proto>          # Create mol
bd mol wisp <proto>          # Create wisp
bd mol bond <proto> <parent> # Attach to existing mol
bd mol squash <id>           # Condense to digest (explicit ID)
bd mol burn <id>             # Discard wisp (explicit ID)
```

### Agent Operations (gt)

```bash
# Hook management (operates on current agent's hook)
gt hook                    # What's on MY hook
gt mol current               # What should I work on next
gt mol progress <id>         # Execution progress of molecule
gt mol attach <bead> <mol>   # Pin molecule to bead
gt mol detach <bead>         # Unpin molecule from bead
gt mol attach-from-mail <id> # Attach from mail message

# Agent lifecycle (operates on agent's attached molecule)
gt mol burn                  # Burn attached molecule (no ID needed)
gt mol squash                # Squash attached molecule (no ID needed)
gt mol step done <step>      # Complete a molecule step
```

**Key distinction**: `bd mol burn/squash <id>` take explicit molecule IDs.
`gt mol burn/squash` operate on the current agent's attached molecule
(auto-detected from working directory).

## Agent Lifecycle

### Polecat Shutdown

```
1. Complete work steps
2. gt mol squash (create digest)
3. Submit to merge queue
4. gt handoff (request shutdown)
5. Wait for Witness to kill session
6. Witness removes worktree + branch
```

### Session Cycling

```
1. Agent notices context filling
2. gt handoff (sends mail to self)
3. Manager kills session
4. Manager starts new session
5. New session reads handoff mail
```

## Environment Variables

Gas Town sets environment variables for each agent session via `config.AgentEnv()`.
These are set in tmux session environment when agents are spawned.

### Core Variables (All Agents)

| Variable | Purpose | Example |
|----------|---------|---------|
| `GT_ROLE` | Agent role type | `mayor`, `deacon`, `librarian`, `witness`, `refinery`, `polecat`, `crew` |
| `GT_ROOT` | Town root directory | `/home/user/gt` |
| `BD_ACTOR` | Agent identity for attribution | `gastown/polecats/toast` |
| `GIT_AUTHOR_NAME` | Commit attribution (same as BD_ACTOR) | `gastown/polecats/toast` |
| `BEADS_DIR` | Beads database location | `/home/user/gt/gastown/.beads` |

### Rig-Level Variables

| Variable | Purpose | Roles |
|----------|---------|-------|
| `GT_RIG` | Rig name | librarian, witness, refinery, polecat, crew |
| `GT_POLECAT` | Polecat worker name | polecat only |
| `GT_CREW` | Crew worker name | crew only |
| `BEADS_AGENT_NAME` | Agent name for beads operations | polecat, crew |

### Other Variables

| Variable | Purpose |
|----------|---------|
| `GIT_AUTHOR_EMAIL` | Workspace owner email (from git config) |
| `GT_TOWN_ROOT` | Override town root detection (manual use) |
| `CLAUDE_RUNTIME_CONFIG_DIR` | Custom Claude settings directory |

### Environment by Role

| Role | Key Variables |
|------|---------------|
| **Mayor** | `GT_ROLE=mayor`, `BD_ACTOR=mayor` |
| **Deacon** | `GT_ROLE=deacon`, `BD_ACTOR=deacon` |
| **Librarian** | `GT_ROLE=librarian`, `GT_RIG=<rig>`, `BD_ACTOR=<rig>/librarian` |
| **Boot** | `GT_ROLE=deacon/boot`, `BD_ACTOR=deacon-boot` |
| **Witness** | `GT_ROLE=witness`, `GT_RIG=<rig>`, `BD_ACTOR=<rig>/witness` |
| **Refinery** | `GT_ROLE=refinery`, `GT_RIG=<rig>`, `BD_ACTOR=<rig>/refinery` |
| **Polecat** | `GT_ROLE=polecat`, `GT_RIG=<rig>`, `GT_POLECAT=<name>`, `BD_ACTOR=<rig>/polecats/<name>` |
| **Crew** | `GT_ROLE=crew`, `GT_RIG=<rig>`, `GT_CREW=<name>`, `BD_ACTOR=<rig>/crew/<name>` |

### Doctor Check

The `gt doctor` command verifies that running tmux sessions have correct
environment variables. Mismatches are reported as warnings:

```
⚠ env-vars: Found 3 env var mismatch(es) across 1 session(s)
    hq-mayor: missing GT_ROOT (expected "/home/user/gt")
```

Fix by restarting sessions: `gt shutdown && gt up`

## Agent Working Directories and Settings

Each agent runs in a specific working directory and has its own Claude settings.
Understanding this hierarchy is essential for proper configuration.

### Working Directories by Role

| Role | Working Directory | Notes |
|------|-------------------|-------|
| **Mayor** | `~/gt/mayor/` | Town-level coordinator, isolated from rigs |
| **Deacon** | `~/gt/deacon/` | Background supervisor daemon |
| **Librarian** | `~/gt/<rig>/librarian/` | Rig-level docs and knowledge operations |
| **Witness** | `~/gt/<rig>/witness/` | No git clone, monitors polecats only |
| **Refinery** | `~/gt/<rig>/refinery/rig/` | Worktree on main branch |
| **Crew** | `~/gt/<rig>/crew/<name>/rig/` | Persistent human workspace clone |
| **Polecat** | `~/gt/<rig>/polecats/<name>/rig/` | Polecat worktree (ephemeral sandbox) |

Note: The per-rig `<rig>/mayor/rig/` directory is NOT a working directory—it's
a git clone that holds the canonical `.beads/` database for that rig.

### Settings File Locations

Settings are installed in gastown-managed parent directories and passed to
Claude Code via the `--settings` flag. This keeps customer repos clean:

```
~/gt/
├── mayor/.claude/settings.json              # Mayor settings (cwd = settings dir)
├── deacon/.claude/settings.json             # Deacon settings (cwd = settings dir)
└── <rig>/
    ├── crew/.claude/settings.json           # Shared by all crew members
    ├── polecats/.claude/settings.json       # Shared by all polecats
    ├── witness/.claude/settings.json        # Witness settings
    └── refinery/.claude/settings.json       # Refinery settings
```

The `--settings` flag loads these as a separate priority tier that merges
additively with any project-level settings in the customer repo.

### CLAUDE.md

Only `~/gt/CLAUDE.md` exists on disk — a minimal identity anchor that prevents
agents from losing their Gas Town identity after context compaction or new sessions.

Full role context (~300-500 lines per role) is injected ephemerally by `gt prime`
via the SessionStart hook. No per-directory CLAUDE.md or AGENTS.md files are created.

**Why no per-directory files?**
- Claude Code traverses upward from CWD for CLAUDE.md — all agents under `~/gt/` find the town-root file
- AGENTS.md (for Codex) uses downward traversal from git root — parent directories are invisible, so per-directory AGENTS.md never worked
- The real context comes from `gt prime`, making on-disk bootstrap pointers redundant

### Customer Repo Files (CLAUDE.md and .claude/)

Gas Town no longer uses git sparse checkout to hide customer repo files. Customer
repositories can have their own `.claude/` directory and `CLAUDE.md` — these are
preserved in all worktrees (crew, polecats, refinery, mayor/rig).

Gas Town's context comes from the town-root `CLAUDE.md` identity anchor
(picked up by all agents via Claude Code's upward directory traversal),
`gt prime` via the SessionStart hook, and the customer repo's own `CLAUDE.md`.
These coexist safely because:

- **`--settings` flag provides Gas Town settings** as a separate tier that merges
  additively with customer project settings, so both coexist cleanly
- **`gt prime` injects role context** ephemerally via SessionStart hook, which is
  additive with the customer's `CLAUDE.md` — both are loaded
- Gas Town settings live in parent directories (not in customer repos), so
  customer `.claude/` files are fully preserved

**Doctor check**: `gt doctor` warns if legacy sparse checkout is still configured.
Run `gt doctor --fix` to remove it. Tracked `settings.json` files in worktrees are
recognized as customer project config and are not flagged as stale.

### Settings Inheritance

Claude Code's settings are layered from multiple sources:

1. `.claude/settings.json` in current working directory (customer project)
2. `.claude/settings.json` in parent directories (traversing up)
3. `~/.claude/settings.json` (user global settings)
4. `--settings <path>` flag (loaded as a separate additive tier)

Gas Town uses the `--settings` flag to inject role-specific settings from
gastown-managed parent directories. This merges additively with customer
project settings rather than overriding them.

### Settings Templates

Gas Town uses two settings templates based on role type:

| Type | Roles | Key Difference |
|------|-------|----------------|
| **Interactive** | Mayor, Crew | Mail injected on `UserPromptSubmit` hook |
| **Autonomous** | Polecat, Witness, Refinery, Deacon | Mail injected on `SessionStart` hook |

Autonomous agents may start without user input, so they need mail checked
at session start. Interactive agents wait for user prompts.

### Troubleshooting

| Problem | Solution |
|---------|----------|
| Agent using wrong settings | Check `gt doctor`, verify `.claude/settings.json` in role parent dir |
| Settings not found | Run `gt install` to recreate settings, or `gt doctor --fix` |
| Source repo settings leaking | Run `gt doctor --fix` to remove legacy sparse checkout |
| Mayor settings affecting polecats | Mayor should run in `mayor/`, not town root |

## CLI Reference

### Town Management

```bash
gt install [path]            # Create town
gt install --git             # With git init
gt doctor                    # Health check
gt doctor --fix              # Auto-repair
```

### Configuration

```bash
# Agent management
gt config agent list [--json]     # List all agents (built-in + custom)
gt config agent get <name>        # Show agent configuration
gt config agent set <name> <cmd>  # Create or update custom agent
gt config agent remove <name>     # Remove custom agent (built-ins protected)

# Default agent
gt config default-agent [name]    # Get or set town default agent
```

**Built-in agents**: `claude`, `gemini`, `codex`, `cursor`, `auggie`, `amp`

**Custom agents**: Define per-town via CLI or JSON:
```bash
gt config agent set claude-glm "claude-glm --model glm-4"
gt config agent set claude "claude-opus"  # Override built-in
gt config default-agent claude-glm       # Set default
```

**Advanced agent config** (`settings/agents.json`):
```json
{
  "version": 1,
  "agents": {
    "opencode": {
      "command": "opencode",
      "args": [],
      "resume_flag": "--session",
      "resume_style": "flag",
      "non_interactive": {
        "subcommand": "run",
        "output_flag": "--format json"
      }
    }
  }
}
```

**Rig-level agents** (`<rig>/settings/config.json`):
```json
{
  "type": "rig-settings",
  "version": 1,
  "agent": "opencode",
  "agents": {
    "opencode": {
      "command": "opencode",
      "args": ["--session"]
    }
  }
}
```

**Agent resolution order**: rig-level → town-level → built-in presets.

For OpenCode autonomous mode, set env var in your shell profile:
```bash
export OPENCODE_PERMISSION='{"*":"allow"}'
```

### Rig Management

```bash
gt rig add <name> <url>
gt rig list
gt rig remove <name>
```

### Convoy Management (Primary Dashboard)

```bash
gt convoy list                          # Dashboard of active convoys
gt convoy status [convoy-id]            # Show progress (🚚 hq-cv-*)
gt convoy create "name" [issues...]     # Create convoy tracking issues
gt convoy create "name" gt-a bd-b --notify mayor/  # With notification
gt convoy list --all                    # Include landed convoys
gt convoy list --status=closed          # Only landed convoys
```

Note: "Swarm" is ephemeral (workers on a convoy's issues). See [Convoys](concepts/convoy.md).

### Work Assignment

```bash
# Standard workflow: convoy first, then sling
gt convoy create "Feature X" gt-abc gt-def
gt sling gt-abc <rig>                    # Assign to polecat
gt sling gt-abc <rig> --agent codex      # Override runtime for this sling/spawn
gt sling <proto> --on gt-def <rig>       # With workflow template

# Quick sling (auto-creates convoy)
gt sling <bead> <rig>                    # Auto-convoy for dashboard visibility
```

Agent overrides:

- `gt start --agent <alias>` overrides the Mayor/Deacon runtime for this launch.
- `gt mayor start|attach|restart --agent <alias>` and `gt deacon start|attach|restart --agent <alias>` do the same.
- `gt start crew <name> --agent <alias>` and `gt crew at <name> --agent <alias>` override the crew worker runtime.

Crew restart modes (important):

- `gt crew refresh` and `gt crew restart` use interactive mode and can trigger approval prompts.
- For non-interactive/no-approval crew restarts, use:

```bash
gt crew stop <name> --rig <rig>
gt crew at <name> --rig <rig> --detached --agent claude
gt nudge <rig>/crew/<name> "Run gt prime, read handoff/context, continue."
```

### Communication

```bash
gt mail inbox
gt mail read <id>
gt mail send <addr> -s "Subject" -m "Body"
gt mail send --human -s "..."    # To overseer
```

### Escalation

```bash
gt escalate "topic"              # Default: MEDIUM severity
gt escalate -s CRITICAL "msg"    # Urgent, immediate attention
gt escalate -s HIGH "msg"        # Important blocker
gt escalate -s MEDIUM "msg" -m "Details..."
```

See [escalation.md](design/escalation.md) for full protocol.

### Sessions

```bash
gt handoff                   # Request cycle (context-aware)
gt handoff --shutdown        # Terminate (polecats)
gt session stop <rig>/<agent>
gt peek <agent>              # Check health
gt nudge <agent> "message"   # Send message to agent
gt seance                    # List discoverable predecessor sessions
gt seance --talk <id>        # Talk to predecessor (full context)
gt seance --talk <id> -p "Where is X?"  # One-shot question
```

**Session Discovery**: Each session has a startup nudge that becomes searchable
in Claude's `/resume` picker:

```
[GAS TOWN] recipient <- sender • timestamp • topic[:mol-id]
```

Example: `[GAS TOWN] gastown/crew/gus <- human • 2025-12-30T15:42 • restart`

**IMPORTANT**: Always use `gt nudge` to send messages to Claude sessions.
Never use raw `tmux send-keys` - it doesn't handle Claude's input correctly.
`gt nudge` uses literal mode + debounce + separate Enter for reliable delivery.

### Emergency

```bash
gt stop --all                # Kill all sessions
gt stop --rig <name>         # Kill rig sessions
```

### Health Check

```bash
gt deacon health-check <agent>   # Send health check ping, track response
gt deacon health-state           # Show health check state for all agents
```

### Merge Queue (MQ)

```bash
gt mq list [rig]             # Show the merge queue
gt mq next [rig]             # Show highest-priority merge request
gt mq submit                 # Submit current branch to merge queue
gt mq status <id>            # Show detailed merge request status
gt mq retry <id>             # Retry a failed merge request
gt mq reject <id>            # Reject a merge request
```

#### Integration Branch Commands

```bash
gt mq integration create <epic-id>              # Create integration branch
gt mq integration create <epic-id> --branch "feat/{title}"  # Custom template
gt mq integration create <epic-id> --base-branch develop   # Non-main base
gt mq integration status <epic-id>              # Show branch status
gt mq integration status <epic-id> --json       # JSON output
gt mq integration land <epic-id>                # Merge to base branch (default: main)
gt mq integration land <epic-id> --dry-run      # Preview only
gt mq integration land <epic-id> --force        # Land with open MRs
gt mq integration land <epic-id> --skip-tests   # Skip test run
```

See [Integration Branches](concepts/integration-branches.md) for the full workflow.

## Beads Commands (bd)

```bash
bd ready                     # Work with no blockers
bd list --status=open
bd list --status=in_progress
bd show <id>
bd create --title="..." --type=task
bd update <id> --status=in_progress
bd close <id>
bd dep add <child> <parent>  # child depends on parent
```

## Command Usage Tips

- **Rig inference** – `gt` commands guess the target rig from the current working directory or spin-up environment (`GT_RIG`, `GT_ROLE`). When you’re outside a rig tree (e.g., `~/gt`) pass `--rig <name>` explicitly (`gt mayor start --rig gastown`), or change directories to the rig root before running gt. `gt rig status` shows both the detected rig and whether a witness/refinery is in the cache; if you pulled from a different repo or your shell lost `GT_RIG`, rerun from the rig directory so the inferred context matches the `role_agents` settings.
- **Agent override** – `--agent <alias>` replaces the runtime that would otherwise be resolved from `role_agents`, town/rig defaults, or the built-in Claude preset. Use it on commands like `gt start`, `gt mayor start`, `gt witness start`, `gt crew <name>`, and `gt refinery start` when experimenting with codex/gemini. After the run, `gt status` (or `gt rig status <name>`) shows `agent_alias`/`agent_info` so you can confirm which runtime actually booted. If the override fails, the command logs `Error: agent '<alias>' not found` so check `gt config agent list` before retrying.
- **bd actor context** – `bd` infers the actor from `GT_ROLE`/`BD_ACTOR`, or from your cwd when you’re inside a rig/role directory. Scripts should export `BD_ACTOR=gastown/crew/gus` or run `cd gastown/crew/gus && bd ready` so the beads backend knows whose queue you’re working. Without that, `bd create` might attribute work to the wrong agent and cause follow-on confusion about assignments or ready lists.
- **Beads DB health** – `bd ready` (`bd list`, `bd show`) expects a healthy `.beads` directory. If you see “Error: no beads database found” or “database locked,” verify you’re under the right rig root, run `bd sync` once to fix a stale lock, and fall back to `bd init`/`bd config` only if nothing else works. Don’t delete `beads/` while sessions are running—we need consistent history for patrol agents.
- **Testing and verification** – For any automation-heavy change, iterate:
  1. Run `gt status --fast` to ensure each agent has the right runtime alias (look at `agent_alias` and `agent_info`, plus the `runtime_drift` warning if it ever appears).
  2. Use `gt rig status <name>` to confirm witness/refinery/witness are running (record `session: gt-witness/gastown` etc.).
  3. If runtime or rig inference still feels flaky, open a `gt` bead and note the exact command plus `GT_RIG`/`GT_ROLE`/`BD_ACTOR` values—this helps the mayor/deacon triage the failure without guessing.
- **Avoiding mistakes** – When a `gt` command fails with “already running,” attach (`gt <role> attach`) instead of restarting; for `bd` commands always check `bd show` before mutating the same issue. If you’re scripting, wrap `gt` calls in `set -e`/`gt status` post-check to make sure the expected sessions exist, re-running `bd ready` after the fact to confirm nothing shifted in the queue.
- **Rig inference** – `gt` commands guess the target rig from the current working directory or the `GT_RIG` environment variable. Pass `--rig <name>` explicitly when working outside the rig tree (`gt mayor start --rig gastown`).
- **Agent override** – `--agent <alias>` replaces the resolved runtime for contacts like `gt start`, `gt mayor start`, `gt witness start`, and `gt crew <name>`. Use it for experiments (e.g., `gt mayor start --agent codex`), but most production flows expect Claude by default; view the remaining runtime with `gt status` (see the agent table for `agent_alias` and `agent_info`).
- **bd actor context** – `bd` reads `GT_ROLE`, `BD_ACTOR`, or the current directory to determine who is speaking. When scripting or running from an automation shell, set `BD_ACTOR` explicitly (`BD_ACTOR=gastown/witness bd ready`) so you land in the right queue.
- **Touchpoints** – Always confirm your `.beads` directory is correct (`bd ready` will warn if it can’t find a database). Use `bd show <id>` to check existing metadata before reassigning staff and lean on `bd update <id> --notes` to log the change.
- **Avoiding mistakes** – For every `gt sling` or `gt start` command that fails, rerun `gt status` and `gt rig status <name>` to verify agent health and role assignment. When hitting ambiguous errors, inspect `gt config role_agents` plus `gt mail identity` to make sure the actor you think you are matches the `GT_ROLE` recorded by the system.

## Patrol Agents

Deacon, Witness, and Refinery run continuous patrol loops using wisps:

| Agent | Patrol Molecule | Responsibility |
|-------|-----------------|----------------|
| **Deacon** | `mol-deacon-patrol` | Agent lifecycle, plugin execution, health checks |
| **Witness** | `mol-witness-patrol` | Monitor polecats, nudge stuck workers |
| **Refinery** | `mol-refinery-patrol` | Process merge queue, review MRs, check integration branches |

```
1. bd mol wisp mol-<role>-patrol
2. Execute steps (check workers, process queue, run plugins)
3. gt mol squash --jitter 10s (or burn if routine)
4. Loop
```

## Plugin Molecules

Plugins are molecules with specific labels:

```json
{
  "id": "mol-security-scan",
  "labels": ["template", "plugin", "witness", "tier:haiku"]
}
```

Patrol molecules bond plugins dynamically:

```bash
bd mol bond mol-security-scan $PATROL_ID --var scope="$SCOPE"
```

## Formula Invocation Patterns

**CRITICAL**: Different formula types require different invocation methods.

### Workflow Formulas (sequential steps, single polecat)

Examples: `shiny`, `shiny-enterprise`, `mol-polecat-work`

```bash
gt sling <formula> --on <bead-id> <target>
gt sling shiny-enterprise --on gt-abc123 gastown
```

### Convoy Formulas (parallel legs, multiple polecats)

Examples: `code-review`

**DO NOT use `gt sling` for convoy formulas!** It fails with "convoy type not supported".

```bash
# Correct invocation - use gt formula run:
gt formula run code-review --pr=123
gt formula run code-review --files="src/*.go"

# Dry run to preview:
gt formula run code-review --pr=123 --dry-run
```

### Identifying Formula Type

```bash
gt formula show <name>   # Shows "Type: convoy" or "Type: workflow"
bd formula list          # Lists formulas by type
```

### Why This Matters

- `gt sling` attempts to cook+pour the formula, which fails for convoy type
- `gt formula run` handles convoy dispatch directly, spawning parallel polecats
- Convoy formulas create multiple polecats (one per leg) + synthesis step

## Common Issues

| Problem | Solution |
|---------|----------|
| Agent in wrong directory | Check cwd, `gt doctor` |
| Beads prefix mismatch | Check `bd show` vs rig config |
| Worktree conflicts | Check worktree state, `gt doctor` |
| Stuck worker | `gt nudge`, then `gt peek` |
| Dirty git state | Commit or discard, then `gt handoff` |

## Architecture Notes

**Bare repo pattern**: `.repo.git/` is bare (no working dir). Refinery and polecats are worktrees sharing refs. Polecat branches visible to refinery immediately.

**Beads as control plane**: No separate orchestrator. Molecule steps ARE beads issues. State transitions are git commits.

**Nondeterministic idempotence**: Any worker can continue any molecule. Steps are atomic checkpoints in beads.

**Convoy tracking**: Convoys track batched work across rigs. A "swarm" is ephemeral - just the workers currently on a convoy's issues. See [Convoys](concepts/convoy.md) for details.
