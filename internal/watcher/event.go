// Package watcher derives each session's live status from two sources: hook
// events over a unix socket (fast, and the only source that can tell "waiting
// for you" from "tool running") and the transcript JSONL (the truth on
// attach, self-healing, and the only source of age and tokens).
//
// Invariant 2: status comes from these structured sources, never from the
// rendered terminal.
package watcher

import (
	"time"
)

// Kind is what happened to a session.
type Kind int

// The events that move a session between statuses. UsageUpdated is orthogonal:
// it carries tokens and never changes the status.
const (
	SessionStarted Kind = iota
	PromptSubmitted
	ToolStarted
	ToolFinished
	PermissionRequested
	TurnEnded
	Idle
	SessionEnded
	UsageUpdated
)

// Tokens is a session's cumulative usage.
type Tokens struct{ In, Out, CacheRead, CacheWrite int }

// add accumulates one response's counters.
func (t *Tokens) add(u Tokens) {
	t.In += u.In
	t.Out += u.Out
	t.CacheRead += u.CacheRead
	t.CacheWrite += u.CacheWrite
}

// Event is one status transition or usage update for one session.
type Event struct {
	SessionID string
	Kind      Kind
	At        time.Time
	Tokens    Tokens // UsageUpdated: cumulative totals
}

// SessionState is what the sidebar shows for a session.
type SessionState struct {
	Status Status
	At     time.Time // when Status was last set; drives the age and newer-wins
	Tokens Tokens
}

// statusFor maps a status-changing Kind to its status. UsageUpdated is absent
// on purpose: it is handled separately so it never regresses a live status.
var statusFor = map[Kind]Status{
	SessionStarted:      StatusIdle,
	PromptSubmitted:     StatusThinking,
	ToolFinished:        StatusThinking,
	ToolStarted:         StatusTool,
	PermissionRequested: StatusWaiting,
	TurnEnded:           StatusDone,
	Idle:                StatusDone,
	SessionEnded:        StatusExited,
}

// Apply folds an event into a session's state. Tokens update regardless of
// order; a status only moves forward in time, so a stale hook or an
// out-of-order tail cannot overwrite fresher state (newer-wins).
func Apply(cur SessionState, ev Event) SessionState {
	if ev.Kind == UsageUpdated {
		cur.Tokens = ev.Tokens
		return cur
	}
	if ev.At.Before(cur.At) {
		return cur
	}
	if status, ok := statusFor[ev.Kind]; ok {
		cur.Status = status
		cur.At = ev.At
	}
	return cur
}
