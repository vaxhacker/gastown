package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/alarm"
	"github.com/steveyegge/gastown/internal/workspace"
)

func init() {
	rootCmd.AddCommand(alarmCmd)
	alarmCmd.AddCommand(alarmAddCmd)
	alarmCmd.AddCommand(alarmListCmd)
	alarmCmd.AddCommand(alarmCancelCmd)

	alarmListCmd.Flags().BoolVar(&alarmListJSON, "json", false, "Output as JSON")
}

var alarmListJSON bool

var alarmCmd = &cobra.Command{
	Use:     "alarm",
	GroupID: GroupComm,
	Short:   "Scheduled nudge reminders",
	Long: `Schedule nudge reminders that fire automatically via the daemon.

Alarms deliver messages via gt nudge at scheduled times. The daemon
checks for due alarms on each heartbeat tick and fires them.

SCHEDULE DSL:
  repeat:<duration>         Recurring every <duration>
  repeat:<duration>@<snap>  Recurring, aligned to snap boundary
  in:<duration>             One-shot after <duration>
  at:<time-expr>            One-shot at specific time

DURATIONS:
  10s, 1m, 5m, 1h, 2h30m, 1d (1d = 24h)

SNAP UNITS:
  @s  second boundary    @d  day boundary (midnight)
  @m  minute boundary    @w  week boundary (Sunday midnight)
  @h  hour boundary      @mon month boundary (1st midnight)

TIME EXPRESSIONS:
  now                     Current time
  now+<duration>          Offset from now
  now-<duration>          Offset before now
  <RFC3339>               Exact timestamp (e.g. 2026-02-22T08:30:00Z)

EXAMPLES:
  gt alarm add repeat:1m@m gastown/witness "status check"
  gt alarm add in:30m mayor/ "review queue depth"
  gt alarm add at:2026-02-22T09:00:00Z gastown/refinery "process queue"
  gt alarm list
  gt alarm cancel a1b2c3d4`,
	RunE: requireSubcommand,
}

var alarmAddCmd = &cobra.Command{
	Use:   "add <schedule> <target> [message]",
	Short: "Add a new alarm",
	Long: `Add a scheduled nudge alarm.

The schedule DSL supports four forms:
  repeat:<duration>         Recurring every <duration>
  repeat:<duration>@<snap>  Recurring, aligned to snap boundary
  in:<duration>             One-shot after <duration>
  at:<time-expr>            One-shot at specific time

Examples:
  gt alarm add repeat:1m@m gastown/witness "status check"
  gt alarm add in:30m mayor/ "review queue depth"
  gt alarm add repeat:5m@h gastown/refinery "check MQ"
  gt alarm add at:now+15m gastown/alpha "wrap up"`,
	Args: cobra.RangeArgs(2, 3),
	RunE: runAlarmAdd,
}

var alarmListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active alarms",
	Args:  cobra.NoArgs,
	RunE:  runAlarmList,
}

var alarmCancelCmd = &cobra.Command{
	Use:   "cancel <alarm-id>",
	Short: "Cancel an alarm",
	Args:  cobra.ExactArgs(1),
	RunE:  runAlarmCancel,
}

func runAlarmAdd(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
	}

	schedule := args[0]
	target := args[1]
	message := "alarm"
	if len(args) > 2 {
		message = args[2]
	}

	a, err := alarm.ParseSchedule(schedule, target, message)
	if err != nil {
		return err
	}

	// Set creator from environment
	if actor := os.Getenv("BD_ACTOR"); actor != "" {
		a.CreatedBy = actor
	}

	if err := alarm.Save(townRoot, a); err != nil {
		return err
	}

	nextLocal := a.NextFireAt.Local().Format("15:04:05")
	fmt.Printf("Alarm %s created → %s\n", a.ID, a.Target)
	fmt.Printf("  Schedule: %s\n", a.Schedule)
	fmt.Printf("  Next fire: %s\n", nextLocal)
	if a.Recurring {
		fmt.Printf("  Interval: %s\n", a.Interval)
		if a.SnapUnit != "" {
			fmt.Printf("  Snap: @%s\n", a.SnapUnit)
		}
	} else {
		fmt.Printf("  Type: one-shot\n")
	}

	return nil
}

func runAlarmList(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
	}

	alarms, err := alarm.List(townRoot)
	if err != nil {
		return err
	}

	if len(alarms) == 0 {
		fmt.Println("No active alarms.")
		return nil
	}

	if alarmListJSON {
		return alarmPrintJSON(alarms)
	}

	now := time.Now()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSCHEDULE\tTARGET\tNEXT FIRE\tFIRES\tSTATUS")
	for _, a := range alarms {
		nextStr := a.NextFireAt.Local().Format("15:04:05")
		untilStr := formatUntil(a.NextFireAt, now)
		status := "ok"
		if a.FailCount > 0 {
			status = fmt.Sprintf("fail(%d)", a.FailCount)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s (%s)\t%d\t%s\n",
			a.ID, a.Schedule, a.Target, nextStr, untilStr, a.FireCount, status)
	}
	return w.Flush()
}

func runAlarmCancel(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
	}

	id := args[0]
	if err := alarm.Cancel(townRoot, id); err != nil {
		return err
	}

	fmt.Printf("Alarm %s cancelled.\n", id)
	return nil
}

// formatUntil returns a human-friendly relative time string.
func formatUntil(target, now time.Time) string {
	d := target.Sub(now)
	if d < 0 {
		return "overdue"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

// alarmPrintJSON outputs alarms as a JSON array.
func alarmPrintJSON(alarms []*alarm.Alarm) error {
	type jsonAlarm struct {
		ID         string    `json:"id"`
		Schedule   string    `json:"schedule"`
		Target     string    `json:"target"`
		Message    string    `json:"message"`
		Recurring  bool      `json:"recurring"`
		NextFireAt time.Time `json:"next_fire_at"`
		FireCount  int       `json:"fire_count"`
		FailCount  int       `json:"fail_count"`
		CreatedAt  time.Time `json:"created_at"`
	}

	var out []jsonAlarm
	for _, a := range alarms {
		out = append(out, jsonAlarm{
			ID:         a.ID,
			Schedule:   a.Schedule,
			Target:     a.Target,
			Message:    a.Message,
			Recurring:  a.Recurring,
			NextFireAt: a.NextFireAt,
			FireCount:  a.FireCount,
			FailCount:  a.FailCount,
			CreatedAt:  a.CreatedAt,
		})
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
