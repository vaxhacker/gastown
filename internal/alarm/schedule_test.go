package alarm

import (
	"strings"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"10s", 10 * time.Second},
		{"1m", time.Minute},
		{"5m", 5 * time.Minute},
		{"1h", time.Hour},
		{"2h30m", 2*time.Hour + 30*time.Minute},
		{"1d", 24 * time.Hour},
		{"2d", 48 * time.Hour},
		{"1d12h", 36 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if err != nil {
				t.Fatalf("parseDuration(%q) error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseScheduleRepeat(t *testing.T) {
	a, err := ParseSchedule("repeat:5m", "gastown/witness", "check status")
	if err != nil {
		t.Fatal(err)
	}
	if !a.Recurring {
		t.Error("expected recurring=true")
	}
	if a.Interval != 5*time.Minute {
		t.Errorf("interval = %v, want 5m", a.Interval)
	}
	if a.SnapUnit != "" {
		t.Errorf("snap = %q, want empty", a.SnapUnit)
	}
	if a.Target != "gastown/witness" {
		t.Errorf("target = %q, want gastown/witness", a.Target)
	}
	if a.Message != "check status" {
		t.Errorf("message = %q, want 'check status'", a.Message)
	}
	if a.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestParseScheduleRepeatSnap(t *testing.T) {
	a, err := ParseSchedule("repeat:1m@m", "mayor/", "ping")
	if err != nil {
		t.Fatal(err)
	}
	if !a.Recurring {
		t.Error("expected recurring=true")
	}
	if a.Interval != time.Minute {
		t.Errorf("interval = %v, want 1m", a.Interval)
	}
	if a.SnapUnit != "m" {
		t.Errorf("snap = %q, want 'm'", a.SnapUnit)
	}
	// NextFireAt should be in the future and aligned to a minute boundary
	if a.NextFireAt.Before(time.Now()) {
		t.Errorf("next_fire_at %v should be in the future", a.NextFireAt)
	}
	if a.NextFireAt.Second() != 0 {
		t.Errorf("next_fire_at second = %d, want 0 (snapped to minute)", a.NextFireAt.Second())
	}
}

func TestParseScheduleIn(t *testing.T) {
	before := time.Now()
	a, err := ParseSchedule("in:30m", "gastown/alpha", "wrap up")
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now()

	if a.Recurring {
		t.Error("expected recurring=false")
	}

	// NextFireAt should be ~30 minutes from now
	expectedMin := before.Add(30 * time.Minute)
	expectedMax := after.Add(30 * time.Minute)
	if a.NextFireAt.Before(expectedMin) || a.NextFireAt.After(expectedMax) {
		t.Errorf("next_fire_at %v not in expected range [%v, %v]",
			a.NextFireAt, expectedMin, expectedMax)
	}
}

func TestParseScheduleAtNow(t *testing.T) {
	before := time.Now()
	a, err := ParseSchedule("at:now+15m", "mayor/", "review")
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now()

	if a.Recurring {
		t.Error("expected recurring=false")
	}

	expectedMin := before.Add(15 * time.Minute)
	expectedMax := after.Add(15 * time.Minute)
	if a.NextFireAt.Before(expectedMin) || a.NextFireAt.After(expectedMax) {
		t.Errorf("next_fire_at %v not in expected range", a.NextFireAt)
	}
}

func TestParseScheduleAtRFC3339(t *testing.T) {
	a, err := ParseSchedule("at:2026-02-22T09:00:00Z", "gastown/refinery", "process")
	if err != nil {
		t.Fatal(err)
	}

	expected, _ := time.Parse(time.RFC3339, "2026-02-22T09:00:00Z")
	if !a.NextFireAt.Equal(expected) {
		t.Errorf("next_fire_at = %v, want %v", a.NextFireAt, expected)
	}
}

func TestParseScheduleInvalid(t *testing.T) {
	tests := []struct {
		schedule string
		errPart  string
	}{
		{"invalid", "invalid schedule format"},
		{"foo:1m", "unknown schedule type"},
		{"repeat:abc", "invalid repeat duration"},
		{"repeat:1m@x", "invalid snap unit"},
		{"in:abc", "invalid delay duration"},
		{"at:yesterday", "expected 'now'"},
	}

	for _, tt := range tests {
		t.Run(tt.schedule, func(t *testing.T) {
			_, err := ParseSchedule(tt.schedule, "mayor/", "test")
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.errPart) {
				t.Errorf("error %q should contain %q", err.Error(), tt.errPart)
			}
		})
	}
}

func TestNextSnappedTime(t *testing.T) {
	// Use a fixed reference time: 2026-02-22 14:37:22 local
	ref := time.Date(2026, 2, 22, 14, 37, 22, 0, time.Local)

	tests := []struct {
		name     string
		interval time.Duration
		snap     string
		check    func(time.Time) bool
		desc     string
	}{
		{
			"1m@m", time.Minute, "m",
			func(t time.Time) bool {
				return t.Second() == 0 && t.After(ref) && t.Before(ref.Add(2*time.Minute))
			},
			"should be next minute boundary",
		},
		{
			"5m@h", 5 * time.Minute, "h",
			func(t time.Time) bool {
				return t.Minute()%5 == 0 && t.Second() == 0 && t.After(ref)
			},
			"should be aligned to 5m from hour start",
		},
		{
			"1h@d", time.Hour, "d",
			func(t time.Time) bool {
				return t.Minute() == 0 && t.Second() == 0 && t.After(ref)
			},
			"should be next hour boundary from day start",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextSnappedTime(ref, tt.interval, tt.snap)
			if !tt.check(got.Local()) {
				t.Errorf("nextSnappedTime(%v, %v, %q) = %v: %s",
					ref, tt.interval, tt.snap, got.Local(), tt.desc)
			}
		})
	}
}

func TestExpandDays(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"1d", "24h"},
		{"2d", "48h"},
		{"1d12h", "24h12h"},
		{"5m", "5m"},
		{"1h30m", "1h30m"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := expandDays(tt.input)
			if got != tt.want {
				t.Errorf("expandDays(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
