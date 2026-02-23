// Package alarm implements scheduled nudge reminders for Gas Town.
//
// Alarms are stored as JSON files in <townRoot>/.runtime/alarms/.
// The daemon checks for due alarms on each heartbeat and fires them
// via gt nudge.
package alarm

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/steveyegge/gastown/internal/constants"
)

// Alarm represents a scheduled nudge reminder.
type Alarm struct {
	// ID is the unique alarm identifier.
	ID string `json:"id"`

	// Schedule is the original schedule string (e.g. "repeat:1m@m").
	Schedule string `json:"schedule"`

	// Target is the gt address to nudge (e.g. "gastown/witness").
	Target string `json:"target"`

	// Message is the nudge message to deliver.
	Message string `json:"message"`

	// Recurring indicates whether this alarm repeats.
	Recurring bool `json:"recurring"`

	// Interval is the repeat interval (for recurring alarms).
	Interval time.Duration `json:"interval,omitempty"`

	// SnapUnit is the snap alignment unit (for recurring alarms with @snap).
	SnapUnit string `json:"snap_unit,omitempty"`

	// NextFireAt is when the alarm should next fire (UTC).
	NextFireAt time.Time `json:"next_fire_at"`

	// CreatedAt is when the alarm was created (UTC).
	CreatedAt time.Time `json:"created_at"`

	// CreatedBy is the agent that created this alarm.
	CreatedBy string `json:"created_by,omitempty"`

	// LastFiredAt is when the alarm last fired (UTC), zero if never fired.
	LastFiredAt time.Time `json:"last_fired_at,omitempty"`

	// FireCount is how many times this alarm has fired.
	FireCount int `json:"fire_count"`

	// FailCount is consecutive failures.
	FailCount int `json:"fail_count"`

	// LastError is the error from the most recent failed fire.
	LastError string `json:"last_error,omitempty"`
}

// alarmsDir returns the path to the alarms storage directory.
func alarmsDir(townRoot string) string {
	return filepath.Join(townRoot, constants.DirRuntime, "alarms")
}

// alarmFile returns the path to a specific alarm's JSON file.
func alarmFile(townRoot, id string) string {
	return filepath.Join(alarmsDir(townRoot), id+".json")
}

// generateID creates a short random alarm ID.
func generateID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Save persists an alarm to disk.
func Save(townRoot string, a *Alarm) error {
	dir := alarmsDir(townRoot)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating alarms dir: %w", err)
	}

	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling alarm: %w", err)
	}

	path := alarmFile(townRoot, a.ID)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing alarm: %w", err)
	}
	return nil
}

// Load reads a single alarm by ID.
func Load(townRoot, id string) (*Alarm, error) {
	path := alarmFile(townRoot, id)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var a Alarm
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("parsing alarm %s: %w", id, err)
	}
	return &a, nil
}

// List returns all alarms sorted by next fire time.
func List(townRoot string) ([]*Alarm, error) {
	dir := alarmsDir(townRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading alarms dir: %w", err)
	}

	var alarms []*Alarm
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		a, err := Load(townRoot, id)
		if err != nil {
			continue // skip malformed files
		}
		alarms = append(alarms, a)
	}

	sort.Slice(alarms, func(i, j int) bool {
		return alarms[i].NextFireAt.Before(alarms[j].NextFireAt)
	})
	return alarms, nil
}

// Cancel removes an alarm by ID.
func Cancel(townRoot, id string) error {
	path := alarmFile(townRoot, id)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("alarm %s not found", id)
		}
		return fmt.Errorf("removing alarm %s: %w", id, err)
	}
	return nil
}

// Due returns all alarms whose NextFireAt is at or before the given time.
func Due(townRoot string, now time.Time) ([]*Alarm, error) {
	all, err := List(townRoot)
	if err != nil {
		return nil, err
	}

	var due []*Alarm
	for _, a := range all {
		if !a.NextFireAt.After(now) {
			due = append(due, a)
		}
	}
	return due, nil
}

// Advance updates the alarm after a successful fire.
// For recurring alarms, computes the next fire time.
// For one-shot alarms, removes the alarm file.
func Advance(townRoot string, a *Alarm, now time.Time) error {
	a.LastFiredAt = now
	a.FireCount++
	a.FailCount = 0
	a.LastError = ""

	if !a.Recurring {
		// One-shot: remove
		return Cancel(townRoot, a.ID)
	}

	// Recurring: compute next fire time
	a.NextFireAt = nextFireTime(a, now)
	return Save(townRoot, a)
}

// RecordFailure updates the alarm after a failed fire attempt.
func RecordFailure(townRoot string, a *Alarm, err error) error {
	a.FailCount++
	a.LastError = err.Error()

	// Bounded backoff: 10s, 30s, 2m, then cap at 2m
	backoffs := []time.Duration{10 * time.Second, 30 * time.Second, 2 * time.Minute}
	idx := a.FailCount - 1
	if idx >= len(backoffs) {
		idx = len(backoffs) - 1
	}
	a.NextFireAt = time.Now().UTC().Add(backoffs[idx])

	return Save(townRoot, a)
}

// nextFireTime computes the next fire time for a recurring alarm.
func nextFireTime(a *Alarm, now time.Time) time.Time {
	if a.SnapUnit != "" {
		return nextSnappedTime(now, a.Interval, a.SnapUnit)
	}
	// No snap: simply add interval to now
	return now.Add(a.Interval)
}

// nextSnappedTime computes the next aligned fire time.
// For example, repeat:1m@m means fire at the next minute boundary.
func nextSnappedTime(now time.Time, interval time.Duration, snapUnit string) time.Time {
	local := now.Local()

	// Truncate to the snap boundary, then step forward by interval
	// until we find a time after now.
	var base time.Time
	switch snapUnit {
	case "s":
		base = local.Truncate(time.Second)
	case "m":
		base = local.Truncate(time.Minute)
	case "h":
		base = local.Truncate(time.Hour)
	case "d":
		y, m, d := local.Date()
		base = time.Date(y, m, d, 0, 0, 0, 0, local.Location())
	case "w":
		y, m, d := local.Date()
		weekday := int(local.Weekday())
		base = time.Date(y, m, d-weekday, 0, 0, 0, 0, local.Location())
	case "mon":
		y, m, _ := local.Date()
		base = time.Date(y, m, 1, 0, 0, 0, 0, local.Location())
	default:
		// Unknown snap unit, fall back to simple interval
		return now.Add(interval)
	}

	// Step forward until past now
	candidate := base
	for !candidate.After(now) {
		candidate = candidate.Add(interval)
	}

	return candidate.UTC()
}
