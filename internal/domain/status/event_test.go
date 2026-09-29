package status_test

import (
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/status"
)

var t0 = time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)

func TestApply_TransitionTable(t *testing.T) {
	tests := []struct {
		kind status.Kind
		want status.Status
	}{
		{status.SessionStarted, status.StatusIdle},
		{status.PromptSubmitted, status.StatusThinking},
		{status.ToolFinished, status.StatusThinking},
		{status.ToolStarted, status.StatusTool},
		{status.PermissionRequested, status.StatusWaiting},
		{status.TurnEnded, status.StatusDone},
		{status.Idle, status.StatusDone},
		{status.SessionEnded, status.StatusExited},
	}
	for _, tt := range tests {
		t.Run(string(tt.want), func(t *testing.T) {
			cur := status.SessionState{Status: status.StatusIdle, At: t0}
			got := status.Apply(cur, status.Event{Kind: tt.kind, At: t0.Add(time.Second)})
			if got.Status != tt.want {
				t.Errorf("Apply(%v) status = %q, want %q", tt.kind, got.Status, tt.want)
			}
		})
	}
}

// Newer-wins: a stale event (a slow hook, an out-of-order tail) must not
// overwrite fresher state.
func TestApply_OlderEventIsIgnored(t *testing.T) {
	cur := status.SessionState{Status: status.StatusWaiting, At: t0}

	got := status.Apply(cur, status.Event{Kind: status.PromptSubmitted, At: t0.Add(-time.Minute)})

	if got.Status != status.StatusWaiting {
		t.Errorf("an older event changed status to %q, want the newer %q kept",
			got.Status, status.StatusWaiting)
	}
}

// An event at exactly the same instant still applies: it is not older.
func TestApply_SameInstantApplies(t *testing.T) {
	cur := status.SessionState{Status: status.StatusIdle, At: t0}

	got := status.Apply(cur, status.Event{Kind: status.ToolStarted, At: t0})

	if got.Status != status.StatusTool {
		t.Errorf("a same-instant event was ignored; status = %q, want tool", got.Status)
	}
}

// Usage carries tokens, never a status; it must leave the status and its
// timestamp alone so it cannot regress a live state.
func TestApply_UsageUpdatedKeepsStatus(t *testing.T) {
	cur := status.SessionState{Status: status.StatusTool, At: t0}
	tok := status.Tokens{In: 100, Out: 20, CacheRead: 5, CacheWrite: 3}

	got := status.Apply(cur, status.Event{Kind: status.UsageUpdated, At: t0.Add(time.Second), Tokens: tok})

	if got.Status != status.StatusTool {
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
	tok := status.Tokens{In: 7}
	got := status.Apply(status.SessionState{}, status.Event{Kind: status.UsageUpdated, At: t0, Tokens: tok})
	if got.Tokens != tok {
		t.Errorf("Tokens = %+v, want %+v recorded on a zero state", got.Tokens, tok)
	}
}

// A cleared pane is a fresh conversation waiting for its first prompt (#316).
func TestApply_ReboundIsIdle_issue316(t *testing.T) {
	cur := status.SessionState{Status: status.StatusDone, At: t0}

	got := status.Apply(cur, status.Event{Kind: status.SessionRebound, At: t0.Add(time.Second)})

	if got.Status != status.StatusIdle {
		t.Errorf("Apply(SessionRebound) status = %q, want %q", got.Status, status.StatusIdle)
	}
}
