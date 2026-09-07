package watcher_test

import (
	"time"

	"github.com/WilsonSousajr/omatty/internal/hooks"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// fakeAdapter is a named Adapter that counts what it was asked and answers
// with canned values, so a test can prove the tailer and the listener parse
// through it rather than through claude's functions (#46).
type fakeAdapter struct {
	Parsed  int
	Entry   watcher.Entry
	Kind    watcher.Kind
	At      time.Time
	Payload hooks.Payload
}

func (f *fakeAdapter) ParseEntry([]byte) (watcher.Entry, bool) {
	f.Parsed++
	return f.Entry, true
}

func (f *fakeAdapter) DeriveKind([]watcher.Entry) (watcher.Kind, time.Time, bool) {
	return f.Kind, f.At, true
}

func (f *fakeAdapter) KindOf(p hooks.Payload) (watcher.Kind, bool) {
	f.Payload = p
	return f.Kind, true
}
