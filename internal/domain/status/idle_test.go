package status_test

import (
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/status"
)

var idleT0 = time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

// The idle sweep's policy (#319), pure since migration step 5.7 (Amendment
// 8, #653): a turn in flight or a question waiting is never idle, whatever
// its age; a settled session is idle once quiet for the threshold.
func TestSweepable_issue319(t *testing.T) {
	for _, c := range []struct {
		name  string
		st    status.Status
		quiet time.Duration
		want  bool
	}{
		{"done, past the threshold", status.StatusDone, 2 * time.Hour, true},
		{"done, inside it", status.StatusDone, 30 * time.Minute, false},
		{"exactly at it", status.StatusIdle, time.Hour, true},
		{"thinking, however old", status.StatusThinking, 72 * time.Hour, false},
		{"running a tool", status.StatusTool, 72 * time.Hour, false},
		{"waiting on the operator", status.StatusWaiting, 72 * time.Hour, false},
	} {
		if got := status.Sweepable(c.st, idleT0, idleT0.Add(c.quiet), time.Hour); got != c.want {
			t.Errorf("%s: Sweepable = %v, want %v", c.name, got, c.want)
		}
	}
}

// The transcript is not the only evidence of use: omatty starting the
// process, or the operator typing into it, is too. The newer of the two wins.
func TestLastActive_isTheNewerRecord_issue319(t *testing.T) {
	transcript, typed := idleT0, idleT0.Add(time.Hour)
	if got := status.LastActive(transcript, typed); !got.Equal(typed) {
		t.Errorf("LastActive = %v, want the typing at %v", got, typed)
	}
	if got := status.LastActive(typed, transcript); !got.Equal(typed) {
		t.Errorf("LastActive = %v, want the transcript at %v", got, typed)
	}
}
