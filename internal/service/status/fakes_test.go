package status_test

import (
	"time"

	"github.com/WilsonSousajr/omatty/internal/infra/hooks"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// fakeAdapter is a named Adapter that counts what it was asked and answers
// with canned values, so a test can prove the tailer and the listener parse
// through it rather than through claude's functions (#46).
type fakeAdapter struct {
	Parsed  int
	Entry   status.Entry
	Kind    status.Kind
	At      time.Time
	Payload hooks.Payload
}

func (f *fakeAdapter) ParseEntry([]byte) (status.Entry, bool) {
	f.Parsed++
	return f.Entry, true
}

func (f *fakeAdapter) DeriveKind([]status.Entry) (status.Kind, time.Time, bool) {
	return f.Kind, f.At, true
}

func (f *fakeAdapter) KindOf(p hooks.Payload) (status.Kind, bool) {
	f.Payload = p
	return f.Kind, true
}
