package status

import dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"

// A session's status vocabulary moved to internal/domain/status (ADR 0001,
// migration step 3.2). These aliases keep every importer compiling while the
// importers move over; migration step 8.1 deletes them. New code imports
// internal/domain/status directly:
//
//	import "github.com/WilsonSousajr/omatty/internal/domain/status"
//	st = status.Apply(st, ev)

// Kind is dstatus.Kind.
type Kind = dstatus.Kind

// Status is dstatus.Status.
type Status = dstatus.Status

// Tokens is dstatus.Tokens.
type Tokens = dstatus.Tokens

// Event is dstatus.Event.
type Event = dstatus.Event

// Entry is dstatus.Entry (Amendment 7, #653).
type Entry = dstatus.Entry

// Adapter is dstatus.Adapter (Amendment 7, #653).
type Adapter = dstatus.Adapter

// SessionState is dstatus.SessionState.
type SessionState = dstatus.SessionState

// The event kinds, as dstatus declares them.
const (
	SessionStarted      = dstatus.SessionStarted
	PromptSubmitted     = dstatus.PromptSubmitted
	ToolStarted         = dstatus.ToolStarted
	ToolFinished        = dstatus.ToolFinished
	PermissionRequested = dstatus.PermissionRequested
	TurnEnded           = dstatus.TurnEnded
	Idle                = dstatus.Idle
	SessionEnded        = dstatus.SessionEnded
	UsageUpdated        = dstatus.UsageUpdated
	SessionRebound      = dstatus.SessionRebound
)

// The statuses, as dstatus declares them.
const (
	StatusIdle     = dstatus.StatusIdle
	StatusThinking = dstatus.StatusThinking
	StatusTool     = dstatus.StatusTool
	StatusWaiting  = dstatus.StatusWaiting
	StatusDone     = dstatus.StatusDone
	StatusError    = dstatus.StatusError
	StatusExited   = dstatus.StatusExited
)

// Apply is dstatus.Apply.
//
//	st = status.Apply(st, ev)
func Apply(cur SessionState, ev Event) SessionState { return dstatus.Apply(cur, ev) }
