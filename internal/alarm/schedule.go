package alarm

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseSchedule parses a schedule DSL string and creates an Alarm.
//
// Supported forms:
//   - repeat:<duration>         — recurring every <duration>
//   - repeat:<duration>@<snap>  — recurring, aligned to snap boundary
//   - in:<duration>             — one-shot after <duration>
//   - at:<time-expr>            — one-shot at specific time
//
// Duration format: Go-style tokens like 10s, 1m, 5m, 1h, 2h30m, 1d (mapped to 24h).
// Snap units: @s, @m, @h, @d, @w, @mon.
// Time expressions: now, now+<dur>, now-<dur>, RFC3339 timestamp.
func ParseSchedule(schedule, target, message string) (*Alarm, error) {
	now := time.Now()

	a := &Alarm{
		ID:        generateID(),
		Schedule:  schedule,
		Target:    target,
		Message:   message,
		CreatedAt: now.UTC(),
	}

	parts := strings.SplitN(schedule, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid schedule format %q: expected <type>:<value>", schedule)
	}

	schedType := parts[0]
	schedValue := parts[1]

	switch schedType {
	case "repeat":
		return parseRepeat(a, schedValue, now)
	case "in":
		return parseIn(a, schedValue, now)
	case "at":
		return parseAt(a, schedValue, now)
	default:
		return nil, fmt.Errorf("unknown schedule type %q: expected repeat, in, or at", schedType)
	}
}

// parseRepeat handles repeat:<duration> and repeat:<duration>@<snap>.
func parseRepeat(a *Alarm, value string, now time.Time) (*Alarm, error) {
	a.Recurring = true

	// Check for snap unit
	if idx := strings.Index(value, "@"); idx >= 0 {
		durStr := value[:idx]
		snapStr := value[idx+1:]

		dur, err := parseDuration(durStr)
		if err != nil {
			return nil, fmt.Errorf("invalid repeat duration %q: %w", durStr, err)
		}

		if !isValidSnap(snapStr) {
			return nil, fmt.Errorf("invalid snap unit %q: expected s, m, h, d, w, or mon", snapStr)
		}

		a.Interval = dur
		a.SnapUnit = snapStr
		a.NextFireAt = nextSnappedTime(now, dur, snapStr)
		return a, nil
	}

	dur, err := parseDuration(value)
	if err != nil {
		return nil, fmt.Errorf("invalid repeat duration %q: %w", value, err)
	}

	a.Interval = dur
	a.NextFireAt = now.Add(dur).UTC()
	return a, nil
}

// parseIn handles in:<duration>.
func parseIn(a *Alarm, value string, now time.Time) (*Alarm, error) {
	dur, err := parseDuration(value)
	if err != nil {
		return nil, fmt.Errorf("invalid delay duration %q: %w", value, err)
	}

	a.Recurring = false
	a.NextFireAt = now.Add(dur).UTC()
	return a, nil
}

// parseAt handles at:<time-expr>.
func parseAt(a *Alarm, value string, now time.Time) (*Alarm, error) {
	t, err := parseTimeExpr(value, now)
	if err != nil {
		return nil, fmt.Errorf("invalid time expression %q: %w", value, err)
	}

	a.Recurring = false
	a.NextFireAt = t.UTC()
	return a, nil
}

// parseDuration parses a Go-style duration with support for "d" (days = 24h).
func parseDuration(s string) (time.Duration, error) {
	// Replace "d" suffix with "h" equivalent (1d = 24h)
	s = expandDays(s)
	return time.ParseDuration(s)
}

// durationPattern matches digits followed by 'd' (not inside another unit).
var durationPattern = regexp.MustCompile(`(\d+)d`)

// expandDays replaces Nd with N*24h in a duration string.
func expandDays(s string) string {
	return durationPattern.ReplaceAllStringFunc(s, func(match string) string {
		numStr := match[:len(match)-1]
		n, err := strconv.Atoi(numStr)
		if err != nil {
			return match
		}
		return fmt.Sprintf("%dh", n*24)
	})
}

// parseTimeExpr parses a time expression:
//   - now
//   - now+<duration>
//   - now-<duration>
//   - RFC3339 timestamp
func parseTimeExpr(s string, now time.Time) (time.Time, error) {
	if s == "now" {
		return now, nil
	}

	if strings.HasPrefix(s, "now+") {
		dur, err := parseDuration(s[4:])
		if err != nil {
			return time.Time{}, err
		}
		return now.Add(dur), nil
	}

	if strings.HasPrefix(s, "now-") {
		dur, err := parseDuration(s[4:])
		if err != nil {
			return time.Time{}, err
		}
		return now.Add(-dur), nil
	}

	// Try RFC3339
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("expected 'now', 'now+<dur>', 'now-<dur>', or RFC3339 timestamp")
	}
	return t, nil
}

// isValidSnap checks if the snap unit is recognized.
func isValidSnap(s string) bool {
	switch s {
	case "s", "m", "h", "d", "w", "mon":
		return true
	default:
		return false
	}
}
