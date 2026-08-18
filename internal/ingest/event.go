package ingest

import (
	"errors"
	"fmt"
	"time"
)

// Event is the call-completion webhook payload sent by the telephony provider.
type Event struct {
	EventID      string    `json:"event_id"`
	CallID       string    `json:"call_id"`
	AccountID    string    `json:"account_id"`
	Status       string    `json:"status"`
	DurationSec  int       `json:"duration_sec"`
	RecordingURL string    `json:"recording_url"`
	OccurredAt   time.Time `json:"occurred_at"`
}

var validStatuses = map[string]bool{
	"completed": true,
	"failed":    true,
	"no_answer": true,
}

// Validate reports whether the payload is well-formed enough to store.
func (e Event) Validate() error {
	if e.EventID == "" {
		return errors.New("event_id is required")
	}
	if e.CallID == "" {
		return errors.New("call_id is required")
	}
	if e.AccountID == "" {
		return errors.New("account_id is required")
	}
	if !validStatuses[e.Status] {
		return fmt.Errorf("status %q is not recognised", e.Status)
	}
	if e.DurationSec < 0 {
		return errors.New("duration_sec must not be negative")
	}
	return nil
}
