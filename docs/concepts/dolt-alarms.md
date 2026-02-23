# Dolt Alarm System

> **Status**: Current reference
> **Scope**: Daemon-managed Dolt server alarming and recovery

---

## Overview

The `gt daemon` continuously monitors the town Dolt server and emits alarms when
the server is degraded. Alarms are sent as mail to:

- `mayor/`
- Every `<rig>/witness` listed in `mayor/rigs.json`

This gives both global and rig-local responders immediate visibility into Dolt
incidents.

## Alarm Types

### 1. Crash Alarm

Subject: `ALERT: Dolt server crashed`

Triggered when daemon finds the tracked Dolt PID dead and begins restart.

### 2. Unhealthy Alarm

Subject: `ALERT: Dolt server unhealthy`

Triggered when connectivity health check fails (`dolt sql -q "SELECT 1"`).

### 3. Read-Only Alarm

Subject: `ALERT: Dolt server entered READ-ONLY mode`

Triggered when read probe succeeds but write probe fails with read-only errors.
This catches the case where reads work but all Beads writes fail.

### 4. Crash-Loop Escalation

Subject: `ESCALATION: Dolt server crash-looping (<N> restarts)`

Triggered when restart attempts exceed `max_restarts_in_window`.
Escalation is sent once per unhealthy window to avoid spam.

## Alert Suppression

To prevent alarm storms during recurring failures, duplicate alarms of the same
type are suppressed for 5 minutes.

- Suppression is per alarm type (`crash`, `unhealthy`, `read_only`)
- Suppression resets after the server passes a healthy cycle
- A new incident after recovery always alerts immediately

## Local Signal File

When Dolt is degraded, daemon writes:

`<town-root>/daemon/DOLT_UNHEALTHY`

JSON payload:

```json
{"reason":"read_only","detail":"...","timestamp":"2026-02-23T01:23:45Z"}
```

Current `reason` values include:

- `server_dead`
- `health_check_failed`
- `read_only`
- `imposter_detected`

The file is removed automatically once health checks pass.

## Operational Checks

```bash
# Check daemon's current Dolt view
gt dolt status

# Inspect daemon log for restart/alert events
tail -n 200 ~/gt/daemon/dolt-server.log

# Inspect unhealthy marker
cat ~/gt/daemon/DOLT_UNHEALTHY
```

## Recovery Playbook

1. Confirm current state with `gt dolt status`.
2. If read-only or unhealthy persists, inspect `dolt-server.log`.
3. Verify disk space and port availability.
4. Restart daemon or Dolt server after correcting root cause.
5. Confirm marker file clears and no new alerts are emitted.
