# GT Alarm Scheduling (MVP)

## Goals

- Add a `gt alarm` subcommand for scheduled reminders.
- Keep the MVP small and operationally reliable.
- Support concise scheduling syntax for recurring reminders.

## Non-Goals (MVP)

- No mail delivery mode (nudge only).
- No timezone configuration surface.
- No Splunk-style `earliest`/`latest` windows.
- No cron or RRULE parser.

## Command Surface

```bash
gt alarm add <schedule> <target> [message]
gt alarm list
gt alarm cancel <alarm-id>
```

### Delivery Mode

- Delivery is always `gt nudge`.
- `target` is required and uses normal gt addressing:
  - `mayor/`
  - `<rig>/witness`
  - `<rig>/refinery`
  - `<rig>/<polecat>`
  - `<rig>/crew/<name>`

## Schedule DSL

Supported schedule forms:

- `repeat:<duration>`
- `repeat:<duration>@<snap>`
- `in:<duration>`
- `at:<time-expr>`

### Duration

Go-style duration tokens:

- `10s`, `1m`, `5m`, `1h`, `2h30m`, `1d` (mapped to `24h`)

### Snap Units

- `@s`, `@m`, `@h`, `@d`, `@w`, `@mon`

Examples:

- `repeat:1m@m`
- `repeat:5m@h`
- `repeat:1h@d`

### Time Expressions

- `now`
- `now+<duration>`
- `now-<duration>`
- RFC3339 timestamp (e.g. `2026-02-22T08:30:00Z`)

Examples:

- `at:now+15m`
- `at:2026-02-22T08:30:00Z`

## Semantics

- `repeat:X` means recurring every `X` from initial fire time.
- `repeat:X@U` means recurring on aligned boundary `U`, not "every X since create".
- Snap uses local server clock boundaries.
- Internally store `next_fire_at` as UTC.
- Display times in local server time for operator readability.

## Runtime Model

- No external daemon required.
- Existing `gt` daemon checks due alarms on a short tick interval.
- For each due alarm:
  1. invoke `gt nudge <target> <message>`
  2. record run result
  3. compute/store next fire time

## Failure Handling

- If nudge fails:
  - mark run failed with error text
  - retry with bounded backoff (e.g. 10s, 30s, 2m)
  - keep alarm active unless cancelled

## Examples

```bash
# Every minute, aligned to the minute boundary
gt alarm add repeat:1m@m cmtestsuite/foo "status check"

# One-shot in 30 minutes
gt alarm add in:30m mayor/ "review queue depth"

# One-shot at exact timestamp
gt alarm add at:2026-02-22T09:00:00Z gastown/refinery "process queue"
```

## Open Questions (Post-MVP)

- Catch-up policy after daemon downtime (`none`, `latest`, `all`).
- Optional jitter support to avoid synchronized spikes.
- Escalation destination after repeated nudge failures.
