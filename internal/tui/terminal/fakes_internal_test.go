package terminal

import (
	"context"
	"log/slog"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// fakeWrite is one write to a child, as the input queue runs it: it reports
// that it ran, can hold the writer until released, and then returns a result
// or panics the way a pre-1.0 emulator might (#734).
type fakeWrite struct {
	name    string
	ran     chan<- string
	release <-chan struct{} // nil: return at once
	result  tea.Msg
	panics  bool
}

// cmd is the write as the tea.Cmd bubbleterm would hand the queue.
func (w fakeWrite) cmd() tea.Cmd {
	return func() tea.Msg {
		w.ran <- w.name
		if w.release != nil {
			<-w.release
		}
		if w.panics {
			panic("emulator exploded writing " + w.name)
		}
		return w.result
	}
}

// recordingHandler keeps every log record, so a test can assert what the
// queue reported and at which level.
type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler            { return h }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

// at returns the records logged at level.
func (h *recordingHandler) at(level slog.Level) []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []slog.Record
	for _, r := range h.records {
		if r.Level == level {
			out = append(out, r)
		}
	}
	return out
}
