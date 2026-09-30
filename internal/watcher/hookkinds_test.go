package watcher_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/hooks"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func TestKindOf_MapsEveryHookEvent(t *testing.T) {
	tests := []struct {
		event, notif string
		want         watcher.Kind
	}{
		{"SessionStart", "", watcher.SessionStarted},
		{"UserPromptSubmit", "", watcher.PromptSubmitted},
		{"PreToolUse", "", watcher.ToolStarted},
		{"PostToolUse", "", watcher.ToolFinished},
		{"PermissionRequest", "", watcher.PermissionRequested},
		{"Notification", "idle_prompt", watcher.Idle},
		{"Notification", "permission_prompt", watcher.PermissionRequested},
		{"Stop", "", watcher.TurnEnded},
		{"SessionEnd", "", watcher.SessionEnded},
	}
	for _, tt := range tests {
		p := hooks.Payload{HookEventName: tt.event, NotificationType: tt.notif}
		got, ok := watcher.KindOf(p)
		if !ok || got != tt.want {
			t.Errorf("KindOf(%s/%s) = (%v, %v), want (%v, true)", tt.event, tt.notif, got, ok, tt.want)
		}
	}
}

func TestKindOf_UnknownEventIsDropped(t *testing.T) {
	if _, ok := watcher.KindOf(hooks.Payload{HookEventName: "PreCompact"}); ok {
		t.Error("KindOf mapped an event omatty does not track")
	}
}

// Regression, issue #316: /clear starts a new conversation and SessionStart's
// source is the only thing that says so. compact is included on the issue's
// word; if it keeps the id, the re-bind is a no-op.
func TestKindOf_ClearedSessionStartIsRebound_issue316(t *testing.T) {
	for _, source := range []string{"clear", "compact"} {
		p := hooks.Payload{HookEventName: "SessionStart", Source: source}
		if got, ok := watcher.KindOf(p); !ok || got != watcher.SessionRebound {
			t.Errorf("KindOf(SessionStart/%s) = (%v, %v), want (SessionRebound, true)", source, got, ok)
		}
	}
}

// The trap #316 names: a desktop Claude Code forked from a pane inherits the
// pane's settings and reports its own new id. It starts with source resume
// (or startup), and must never take the pane over.
func TestKindOf_ForkedSessionStartIsNotRebound_issue316(t *testing.T) {
	for _, source := range []string{"resume", "startup", ""} {
		p := hooks.Payload{HookEventName: "SessionStart", Source: source}
		if got, ok := watcher.KindOf(p); !ok || got != watcher.SessionStarted {
			t.Errorf("KindOf(SessionStart/%q) = (%v, %v), want (SessionStarted, true)", source, got, ok)
		}
	}
}
