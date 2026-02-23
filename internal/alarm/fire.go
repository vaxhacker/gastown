package alarm

import (
	"fmt"
	"os/exec"
	"time"
)

// FireDue checks for due alarms and fires them via gt nudge.
// This is called from the daemon heartbeat loop.
// gtPath should be the resolved path to the gt binary.
func FireDue(townRoot, gtPath string, logf func(string, ...interface{})) {
	now := time.Now().UTC()

	due, err := Due(townRoot, now)
	if err != nil {
		logf("Alarm: failed to list due alarms: %v", err)
		return
	}

	if len(due) == 0 {
		return
	}

	logf("Alarm: %d alarm(s) due", len(due))

	for _, a := range due {
		logf("Alarm: firing %s → %s %q", a.ID, a.Target, a.Message)

		if err := fireNudge(gtPath, a.Target, a.Message, a.ID); err != nil {
			logf("Alarm: fire %s failed: %v", a.ID, err)
			if rfErr := RecordFailure(townRoot, a, err); rfErr != nil {
				logf("Alarm: failed to record failure for %s: %v", a.ID, rfErr)
			}
			continue
		}

		logf("Alarm: fired %s successfully (fire #%d)", a.ID, a.FireCount+1)
		if err := Advance(townRoot, a, now); err != nil {
			logf("Alarm: failed to advance %s: %v", a.ID, err)
		}
	}
}

// fireNudge invokes `gt nudge <target> <message>` as a subprocess.
func fireNudge(gtPath, target, message, alarmID string) error {
	msg := fmt.Sprintf("[alarm:%s] %s", alarmID, message)

	cmd := exec.Command(gtPath, "nudge", target, msg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}
