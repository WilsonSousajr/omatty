package status_test

import (
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/service/status"
)

func TestKindOf_MapsEveryHookEvent(t *testing.T) {
	tests := []struct {
		event, notif string
		want         dstatus.Kind
	}{
		{"SessionStart", "", dstatus.SessionStarted},
		{"UserPromptSubmit", "", dstatus.PromptSubmitted},
		{"PreToolUse", "", dstatus.ToolStarted},
		{"PostToolUse", "", dstatus.ToolFinished},
		{"PermissionRequest", "", dstatus.PermissionRequested},
		{"Notification", "idle_prompt", dstatus.Idle},
		{"Notification", "permission_prompt", dstatus.PermissionRequested},
		{"Stop", "", dstatus.TurnEnded},
		{"SessionEnd", "", dstatus.SessionEnded},
	}
	for _, tt := range tests {
		p := dstatus.HookPayload{HookEventName: tt.event, NotificationType: tt.notif}
		got, ok := status.KindOf(p)
		if !ok || got != tt.want {
			t.Errorf("KindOf(%s/%s) = (%v, %v), want (%v, true)", tt.event, tt.notif, got, ok, tt.want)
		}
	}
}

func TestKindOf_UnknownEventIsDropped(t *testing.T) {
	if _, ok := status.KindOf(dstatus.HookPayload{HookEventName: "PreCompact"}); ok {
		t.Error("KindOf mapped an event omatty does not track")
	}
}

// Regression, issue #316: /clear starts a new conversation and SessionStart's
// source is the only thing that says so. compact is included on the issue's
// word; if it keeps the id, the re-bind is a no-op.
func TestKindOf_ClearedSessionStartIsRebound_issue316(t *testing.T) {
	for _, source := range []string{"clear", "compact"} {
		p := dstatus.HookPayload{HookEventName: "SessionStart", Source: source}
		if got, ok := status.KindOf(p); !ok || got != dstatus.SessionRebound {
			t.Errorf("KindOf(SessionStart/%s) = (%v, %v), want (SessionRebound, true)", source, got, ok)
		}
	}
}

// The trap #316 names: a desktop Claude Code forked from a pane inherits the
// pane's settings and reports its own new id. It starts with source resume
// (or startup), and must never take the pane over.
func TestKindOf_ForkedSessionStartIsNotRebound_issue316(t *testing.T) {
	for _, source := range []string{"resume", "startup", ""} {
		p := dstatus.HookPayload{HookEventName: "SessionStart", Source: source}
		if got, ok := status.KindOf(p); !ok || got != dstatus.SessionStarted {
			t.Errorf("KindOf(SessionStart/%q) = (%v, %v), want (SessionStarted, true)", source, got, ok)
		}
	}
}
