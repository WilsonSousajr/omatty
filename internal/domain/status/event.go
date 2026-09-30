// Package status is what omatty knows about a session's state: the Kind of
// each event a transcript or a hook reports, the Status a card shows, the
// token usage, the hook payload, and Apply, which folds an event into a
// session's state. It is pure; reading transcripts and hooks is
// internal/service/status's business (invariant 2: status comes from JSONL and hooks,
// never from the screen).
//
//	st = status.Apply(st, status.Event{SessionID: id, Kind: status.TurnEnded, At: now})
package status

import (
	"time"
)

// Kind is what happened to a session.
type Kind int

// The events that move a session between statuses. UsageUpdated is orthogonal:
// it carries tokens and never changes the status. SessionRebound is a
// SessionStart for a new conversation in a running pane - a /clear - and
// names that pane in Event.Owner (#316).
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
	SessionRebound
)

// Tokens is a session's cumulative usage.
type Tokens struct{ In, Out, CacheRead, CacheWrite int }

// Add accumulates one response's counters.
func (t *Tokens) Add(u Tokens) {
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
	// Owner is the registry id of the pane whose claude sent a hook event,
	// from hooks.SessionEnv; empty for the tailer and for a claude omatty did
	// not launch. SessionID is the conversation, which /clear changes (#316).
	Owner string
	// Hook is true on an event the socket listener produced, false on the
	// tailer's. The tailer's PromptSubmitted also fires on tool results, so a
	// consumer that needs "a prompt was just submitted" must ask this (#311).
	Hook bool
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
	SessionRebound:      StatusIdle,
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
