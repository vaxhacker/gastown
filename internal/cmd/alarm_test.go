package cmd

import (
	"bytes"
	"testing"
	"time"
)

func TestFormatUntil(t *testing.T) {
	now := time.Date(2026, 2, 22, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		target time.Time
		want   string
	}{
		{"overdue", now.Add(-time.Minute), "overdue"},
		{"seconds", now.Add(45 * time.Second), "45s"},
		{"minutes_seconds", now.Add(3*time.Minute + 15*time.Second), "3m15s"},
		{"hours_minutes", now.Add(2*time.Hour + 10*time.Minute), "2h10m"},
		{"exactly_now", now, "0s"}, // zero diff is not overdue
		{"one_second", now.Add(time.Second), "1s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatUntil(tt.target, now)
			if got != tt.want {
				t.Errorf("formatUntil() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAlarmCmdStructure(t *testing.T) {
	// Verify alarm command has the expected subcommands registered
	subs := alarmCmd.Commands()
	names := make(map[string]bool)
	for _, c := range subs {
		names[c.Name()] = true
	}

	required := []string{"add", "list", "cancel"}
	for _, name := range required {
		if !names[name] {
			t.Errorf("alarm subcommand %q not registered", name)
		}
	}
}

func TestAlarmCmdGroupID(t *testing.T) {
	if alarmCmd.GroupID != GroupComm {
		t.Errorf("alarm GroupID = %q, want %q", alarmCmd.GroupID, GroupComm)
	}
}

func TestAlarmAddArgsValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no_args", []string{}, true},
		{"one_arg", []string{"repeat:1m"}, true},
		{"two_args_valid", []string{"repeat:1m", "mayor/"}, false},
		{"three_args_valid", []string{"repeat:1m", "mayor/", "hello"}, false},
		{"four_args_too_many", []string{"repeat:1m", "mayor/", "hello", "extra"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := alarmAddCmd.Args(alarmAddCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestAlarmListArgsValidation(t *testing.T) {
	// list accepts no args
	if err := alarmListCmd.Args(alarmListCmd, []string{}); err != nil {
		t.Errorf("list with no args should succeed: %v", err)
	}
	if err := alarmListCmd.Args(alarmListCmd, []string{"extra"}); err == nil {
		t.Error("list with args should fail")
	}
}

func TestAlarmCancelArgsValidation(t *testing.T) {
	// cancel requires exactly 1 arg
	if err := alarmCancelCmd.Args(alarmCancelCmd, []string{}); err == nil {
		t.Error("cancel with no args should fail")
	}
	if err := alarmCancelCmd.Args(alarmCancelCmd, []string{"id1"}); err != nil {
		t.Errorf("cancel with one arg should succeed: %v", err)
	}
	if err := alarmCancelCmd.Args(alarmCancelCmd, []string{"id1", "id2"}); err == nil {
		t.Error("cancel with two args should fail")
	}
}

func TestAlarmListJSONFlag(t *testing.T) {
	f := alarmListCmd.Flags().Lookup("json")
	if f == nil {
		t.Fatal("--json flag not registered on alarm list")
	}
	if f.DefValue != "false" {
		t.Errorf("--json default = %q, want 'false'", f.DefValue)
	}
}

func TestAlarmRequiresSubcommand(t *testing.T) {
	// Parent command with no subcommand should return an error.
	var buf bytes.Buffer
	alarmCmd.SetOut(&buf)
	alarmCmd.SetErr(&buf)
	err := alarmCmd.RunE(alarmCmd, []string{})
	if err == nil {
		t.Fatal("alarm without subcommand should return error")
	}
}
