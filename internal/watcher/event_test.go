package watcher_test

import (
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

var t0 = time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)

func TestApply_TransitionTable(t *testing.T) {
	tests := []struct {
		kind watcher.Kind
		want registry.Status
	}{
		{watcher.SessionStarted, registry.StatusIdle},
		{watcher.PromptSubmitted, registry.StatusThinking},
		{watcher.ToolFinished, registry.StatusThinking},
		{watcher.ToolStarted, registry.StatusTool},
		{watcher.PermissionRequested, registry.StatusWaiting},
		{watcher.TurnEnded, registry.StatusDone},
		{watcher.Idle, registry.StatusDone},
		{watcher.SessionEnded, registry.StatusExited},
	}
	for _, tt := range tests {
		t.Run(tt.want.String(), func(t *testing.T) {
			cur := watcher.SessionState{Status: registry.StatusIdle, At: t0}
			got := watcher.Apply(cur, watcher.Event{Kind: tt.kind, At: t0.Add(time.Second)})
			if got.Status != tt.want {
				t.Errorf("Apply(%v) status = %q, want %q", tt.kind, got.Status, tt.want)
			}
		})
	}
}

// Newer-wins: a stale event (a slow hook, an out-of-order tail) must not
// overwrite fresher state.
func TestApply_OlderEventIsIgnored(t *testing.T) {
	cur := watcher.SessionState{Status: registry.StatusWaiting, At: t0}

	got := watcher.Apply(cur, watcher.Event{Kind: watcher.PromptSubmitted, At: t0.Add(-time.Minute)})

	if got.Status != registry.StatusWaiting {
		t.Errorf("an older event changed status to %q, want the newer %q kept",
			got.Status, registry.StatusWaiting)
	}
}

// An event at exactly the same instant still applies: it is not older.
func TestApply_SameInstantApplies(t *testing.T) {
	cur := watcher.SessionState{Status: registry.StatusIdle, At: t0}

	got := watcher.Apply(cur, watcher.Event{Kind: watcher.ToolStarted, At: t0})

	if got.Status != registry.StatusTool {
		t.Errorf("a same-instant event was ignored; status = %q, want tool", got.Status)
	}
}

// Usage carries tokens, never a status; it must leave the status and its
// timestamp alone so it cannot regress a live state.
func TestApply_UsageUpdatedKeepsStatus(t *testing.T) {
	cur := watcher.SessionState{Status: registry.StatusTool, At: t0}
	tok := watcher.Tokens{In: 100, Out: 20, CacheRead: 5, CacheWrite: 3}

	got := watcher.Apply(cur, watcher.Event{Kind: watcher.UsageUpdated, At: t0.Add(time.Second), Tokens: tok})

	if got.Status != registry.StatusTool {
		t.Errorf("UsageUpdated changed status to %q, want tool unchanged", got.Status)
	}
	if got.Tokens != tok {
		t.Errorf("Tokens = %+v, want %+v", got.Tokens, tok)
	}
	if !got.At.Equal(t0) {
		t.Errorf("UsageUpdated advanced the status timestamp to %v, want %v kept", got.At, t0)
	}
}

func TestApply_UsageOnFreshStateStillRecordsTokens(t *testing.T) {
	tok := watcher.Tokens{In: 7}
	got := watcher.Apply(watcher.SessionState{}, watcher.Event{Kind: watcher.UsageUpdated, At: t0, Tokens: tok})
	if got.Tokens != tok {
		t.Errorf("Tokens = %+v, want %+v recorded on a zero state", got.Tokens, tok)
	}
}
