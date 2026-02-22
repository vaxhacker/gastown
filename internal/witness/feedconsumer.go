package witness

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"
)

// FeedEvent represents a parsed event from .events.jsonl.
type FeedEvent struct {
	Timestamp  time.Time
	Type       string
	Actor      string
	Payload    map[string]interface{}
	Visibility string
	Raw        string
}

// rawFeedEvent is the JSON structure in .events.jsonl.
type rawFeedEvent struct {
	Timestamp  string                 `json:"ts"`
	Source     string                 `json:"source"`
	Type       string                 `json:"type"`
	Actor      string                 `json:"actor"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
	Visibility string                 `json:"visibility"`
}

// FeedConsumer tails .events.jsonl and emits parsed events.
// It skips historical events and only delivers new ones that arrive
// after the consumer starts.
type FeedConsumer struct {
	eventsPath string
	events     chan FeedEvent
	cancel     context.CancelFunc
}

// NewFeedConsumer creates a consumer that tails the given events file.
// Only events written after the consumer starts are delivered.
func NewFeedConsumer(eventsPath string) (*FeedConsumer, error) {
	file, err := os.Open(eventsPath)
	if err != nil {
		return nil, err
	}

	// Seek to end — only deliver new events
	if _, err := file.Seek(0, 2); err != nil {
		file.Close()
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	fc := &FeedConsumer{
		eventsPath: eventsPath,
		events:     make(chan FeedEvent, 100),
		cancel:     cancel,
	}

	go fc.tail(ctx, file)

	return fc, nil
}

// Events returns the channel of parsed feed events.
func (fc *FeedConsumer) Events() <-chan FeedEvent {
	return fc.events
}

// Close stops the consumer.
func (fc *FeedConsumer) Close() {
	fc.cancel()
}

// tail polls the file for new lines. Uses a polling approach (100ms)
// for cross-platform compatibility.
func (fc *FeedConsumer) tail(ctx context.Context, file *os.File) {
	defer close(fc.events)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for scanner.Scan() {
				line := scanner.Text()
				if event, ok := parseFeedLine(line); ok {
					select {
					case fc.events <- event:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}
}

// parseFeedLine parses a single JSON line from .events.jsonl.
func parseFeedLine(line string) (FeedEvent, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return FeedEvent{}, false
	}

	var raw rawFeedEvent
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return FeedEvent{}, false
	}

	ts, err := time.Parse(time.RFC3339, raw.Timestamp)
	if err != nil {
		ts = time.Now()
	}

	return FeedEvent{
		Timestamp:  ts,
		Type:       raw.Type,
		Actor:      raw.Actor,
		Payload:    raw.Payload,
		Visibility: raw.Visibility,
		Raw:        line,
	}, true
}

// WitnessRelevantEventTypes lists event types the witness should react to.
var WitnessRelevantEventTypes = map[string]bool{
	"done":          true, // Polecat completed work (POLECAT_DONE equivalent)
	"merged":        true, // Branch merged by refinery
	"merge_failed":  true, // Merge failed
	"spawn":         true, // New polecat spawned
	"kill":          true, // Polecat killed
	"session_death": true, // Session died unexpectedly
	"mass_death":    true, // Multiple sessions died
	"mail":          true, // Mail sent (may contain HELP, etc.)
	"sling":         true, // Work assigned to polecat
	"hook":          true, // Bead hooked
}

// IsWitnessRelevant returns true if the event type is one the witness should react to.
func IsWitnessRelevant(eventType string) bool {
	return WitnessRelevantEventTypes[eventType]
}

// PayloadString extracts a string value from a feed event payload.
func PayloadString(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}
	if v, ok := payload[key].(string); ok {
		return v
	}
	return ""
}
