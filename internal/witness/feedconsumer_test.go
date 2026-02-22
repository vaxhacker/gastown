package witness

import (
	"testing"
	"time"
)

func TestParseFeedLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantOK  bool
		wantType string
		wantActor string
	}{
		{
			name:    "valid done event",
			line:    `{"ts":"2026-02-22T15:30:45Z","source":"gt","type":"done","actor":"gastown/polecats/nux","payload":{"bead":"gt-abc","branch":"polecat/nux/gt-abc"},"visibility":"feed"}`,
			wantOK:  true,
			wantType: "done",
			wantActor: "gastown/polecats/nux",
		},
		{
			name:    "valid merged event",
			line:    `{"ts":"2026-02-22T16:00:00Z","source":"gt","type":"merged","actor":"gastown/refinery","payload":{"worker":"nux","branch":"polecat/nux/gt-xyz"},"visibility":"feed"}`,
			wantOK:  true,
			wantType: "merged",
			wantActor: "gastown/refinery",
		},
		{
			name:    "valid patrol event",
			line:    `{"ts":"2026-02-22T16:05:00Z","source":"gt","type":"patrol_started","actor":"gastown/witness","payload":{"rig":"gastown"},"visibility":"feed"}`,
			wantOK:  true,
			wantType: "patrol_started",
			wantActor: "gastown/witness",
		},
		{
			name:   "empty line",
			line:   "",
			wantOK: false,
		},
		{
			name:   "invalid JSON",
			line:   "not json",
			wantOK: false,
		},
		{
			name:   "whitespace only",
			line:   "   ",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, ok := parseFeedLine(tt.line)
			if ok != tt.wantOK {
				t.Errorf("parseFeedLine() ok = %v, want %v", ok, tt.wantOK)
				return
			}
			if !ok {
				return
			}
			if event.Type != tt.wantType {
				t.Errorf("parseFeedLine() type = %q, want %q", event.Type, tt.wantType)
			}
			if event.Actor != tt.wantActor {
				t.Errorf("parseFeedLine() actor = %q, want %q", event.Actor, tt.wantActor)
			}
			if event.Timestamp.IsZero() {
				t.Error("parseFeedLine() timestamp should not be zero")
			}
		})
	}
}

func TestParseFeedLineTimestamp(t *testing.T) {
	line := `{"ts":"2026-02-22T15:30:45Z","source":"gt","type":"done","actor":"x","visibility":"feed"}`
	event, ok := parseFeedLine(line)
	if !ok {
		t.Fatal("expected ok")
	}

	expected := time.Date(2026, 2, 22, 15, 30, 45, 0, time.UTC)
	if !event.Timestamp.Equal(expected) {
		t.Errorf("timestamp = %v, want %v", event.Timestamp, expected)
	}
}

func TestParseFeedLineBadTimestamp(t *testing.T) {
	line := `{"ts":"not-a-timestamp","source":"gt","type":"done","actor":"x","visibility":"feed"}`
	event, ok := parseFeedLine(line)
	if !ok {
		t.Fatal("expected ok even with bad timestamp")
	}
	// Should fall back to time.Now()
	if event.Timestamp.IsZero() {
		t.Error("timestamp should not be zero (should fall back to now)")
	}
}

func TestIsWitnessRelevant(t *testing.T) {
	relevant := []string{"done", "merged", "merge_failed", "spawn", "kill", "session_death", "mass_death", "mail", "sling", "hook"}
	for _, eventType := range relevant {
		if !IsWitnessRelevant(eventType) {
			t.Errorf("expected %q to be witness-relevant", eventType)
		}
	}

	irrelevant := []string{"patrol_started", "patrol_complete", "polecat_checked", "handoff", "boot", "halt", "session_start"}
	for _, eventType := range irrelevant {
		if IsWitnessRelevant(eventType) {
			t.Errorf("expected %q to NOT be witness-relevant", eventType)
		}
	}
}

func TestPayloadString(t *testing.T) {
	payload := map[string]interface{}{
		"bead":   "gt-abc",
		"branch": "main",
		"count":  float64(42),
	}

	if got := PayloadString(payload, "bead"); got != "gt-abc" {
		t.Errorf("PayloadString(bead) = %q, want %q", got, "gt-abc")
	}
	if got := PayloadString(payload, "branch"); got != "main" {
		t.Errorf("PayloadString(branch) = %q, want %q", got, "main")
	}
	if got := PayloadString(payload, "missing"); got != "" {
		t.Errorf("PayloadString(missing) = %q, want empty", got)
	}
	if got := PayloadString(payload, "count"); got != "" {
		t.Errorf("PayloadString(count) = %q, want empty (float64 not string)", got)
	}
	if got := PayloadString(nil, "key"); got != "" {
		t.Errorf("PayloadString(nil, key) = %q, want empty", got)
	}
}
