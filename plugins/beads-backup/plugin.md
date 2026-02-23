+++
name = "beads-backup"
description = "Create periodic compressed backups of beads databases and metadata"
version = 1

[gate]
type = "cooldown"
duration = "1h"

[tracking]
labels = ["plugin:beads-backup", "category:backup"]
digest = true

[execution]
timeout = "10m"
notify_on_failure = true
severity = "medium"
+++

# Beads Database Backup

Creates a timestamped backup snapshot for the town's beads data, including:
- `.dolt-data/` (authoritative Dolt database storage)
- town-level `.beads/`
- each rig's `mayor/rig/.beads/` (if present)

Backups are written to `"$GT_TOWN_ROOT/backups/beads/"` and old snapshots are pruned.

## Step 1: Preconditions

```bash
set -euo pipefail

if [ -z "${GT_TOWN_ROOT:-}" ]; then
  echo "SKIP: GT_TOWN_ROOT is not set in this session"
  exit 0
fi

if [ ! -d "$GT_TOWN_ROOT" ]; then
  echo "SKIP: GT_TOWN_ROOT does not exist: $GT_TOWN_ROOT"
  exit 0
fi
```

## Step 2: Create timestamped backup directory

```bash
TOWN_ROOT="$GT_TOWN_ROOT"
BACKUP_ROOT="$TOWN_ROOT/backups/beads"
TIMESTAMP="$(date -u +%Y%m%d-%H%M%S)"
SNAPSHOT_DIR="$BACKUP_ROOT/$TIMESTAMP"

mkdir -p "$SNAPSHOT_DIR"
```

## Step 3: Backup town-level data

```bash
if [ -d "$TOWN_ROOT/.dolt-data" ]; then
  tar -C "$TOWN_ROOT" -czf "$SNAPSHOT_DIR/dolt-data.tgz" ".dolt-data"
else
  echo "WARN: no .dolt-data directory at $TOWN_ROOT/.dolt-data"
fi

if [ -d "$TOWN_ROOT/.beads" ]; then
  tar -C "$TOWN_ROOT" -czf "$SNAPSHOT_DIR/town-beads.tgz" ".beads"
else
  echo "WARN: no town .beads directory at $TOWN_ROOT/.beads"
fi
```

## Step 4: Backup rig-level beads metadata

```bash
RIGS_JSON="$(gt rig list --json 2>/dev/null || echo '[]')"

RIG_COUNT=0
RIG_BACKUPS=0

for RIG in $(echo "$RIGS_JSON" | jq -r '.[].name // empty'); do
  RIG_COUNT=$((RIG_COUNT + 1))
  RIG_BEADS="$TOWN_ROOT/$RIG/mayor/rig/.beads"

  if [ -d "$RIG_BEADS" ]; then
    tar -C "$TOWN_ROOT/$RIG/mayor/rig" -czf "$SNAPSHOT_DIR/${RIG}-beads.tgz" ".beads"
    RIG_BACKUPS=$((RIG_BACKUPS + 1))
  fi
done
```

## Step 5: Write manifest and checksums

```bash
{
  echo "timestamp=$TIMESTAMP"
  echo "town_root=$TOWN_ROOT"
  echo "created_at_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "rigs_seen=$RIG_COUNT"
  echo "rig_backups=$RIG_BACKUPS"
} > "$SNAPSHOT_DIR/manifest.txt"

(
  cd "$SNAPSHOT_DIR"
  sha256sum ./*.tgz > SHA256SUMS.txt 2>/dev/null || true
)
```

## Step 6: Prune old backups (retain at least 48 hours)

```bash
RETENTION_HOURS=48
NOW_EPOCH="$(date -u +%s)"

find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d | while read -r SNAP_DIR; do
  SNAP_NAME="$(basename "$SNAP_DIR")"
  SNAP_EPOCH="$(date -u -d "${SNAP_NAME:0:8} ${SNAP_NAME:9:2}:${SNAP_NAME:11:2}:${SNAP_NAME:13:2}" +%s 2>/dev/null || true)"

  if [ -n "$SNAP_EPOCH" ] && [ $((NOW_EPOCH - SNAP_EPOCH)) -gt $((RETENTION_HOURS * 3600)) ]; then
    rm -rf "$SNAP_DIR"
  fi
done
```

## Record Result

```bash
ARCHIVE_COUNT=$(find "$SNAPSHOT_DIR" -maxdepth 1 -name '*.tgz' | wc -l | tr -d ' ')
TOTAL_SIZE=$(du -sh "$SNAPSHOT_DIR" | awk '{print $1}')
SUMMARY="snapshot=$TIMESTAMP archives=$ARCHIVE_COUNT size=$TOTAL_SIZE rig_backups=$RIG_BACKUPS"
echo "beads-backup: $SUMMARY"
```

On success:
```bash
bd create "beads-backup: $SUMMARY" -t chore --ephemeral \
  -l type:plugin-run,plugin:beads-backup,result:success \
  -d "$SUMMARY" --silent 2>/dev/null || true
```

On failure:
```bash
bd create "beads-backup: FAILED" -t chore --ephemeral \
  -l type:plugin-run,plugin:beads-backup,result:failure \
  -d "beads-backup failed on $(date -u +%Y-%m-%dT%H:%M:%SZ)" --silent 2>/dev/null || true

gt escalate "Plugin FAILED: beads-backup" \
  --severity medium \
  --reason "Periodic beads backup failed"
```
