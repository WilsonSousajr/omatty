package agent_test

import (
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/status"
)

// fakeAdapter is a status.Adapter that reads nothing, for a profile whose
// adapter a test needs present but never calls.
type fakeAdapter struct{}

func (fakeAdapter) ParseEntry([]byte) (status.Entry, bool) { return status.Entry{}, false }
func (fakeAdapter) DeriveKind([]status.Entry) (status.Kind, time.Time, bool) {
	return 0, time.Time{}, false
}
func (fakeAdapter) KindOf(status.HookPayload) (status.Kind, bool) { return 0, false }
