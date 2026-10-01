package status_test

import (
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"time"
)

// fakeAdapter is a named Adapter that counts what it was asked and answers
// with canned values, so a test can prove the tailer and the listener parse
// through it rather than through claude's functions (#46).
type fakeAdapter struct {
	Parsed  int
	Entry   dstatus.Entry
	Kind    dstatus.Kind
	At      time.Time
	Payload dstatus.HookPayload
}

func (f *fakeAdapter) ParseEntry([]byte) (dstatus.Entry, bool) {
	f.Parsed++
	return f.Entry, true
}

func (f *fakeAdapter) DeriveKind([]dstatus.Entry) (dstatus.Kind, time.Time, bool) {
	return f.Kind, f.At, true
}

func (f *fakeAdapter) KindOf(p dstatus.HookPayload) (dstatus.Kind, bool) {
	f.Payload = p
	return f.Kind, true
}
