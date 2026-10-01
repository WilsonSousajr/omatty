package status

import (
	"sort"

	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
)

// kindByEvent maps a hook event name to its status event. Notification is
// absent here because it depends on notification_type.
var kindByEvent = map[string]dstatus.Kind{
	"SessionStart":      dstatus.SessionStarted,
	"UserPromptSubmit":  dstatus.PromptSubmitted,
	"PreToolUse":        dstatus.ToolStarted,
	"PostToolUse":       dstatus.ToolFinished,
	"PermissionRequest": dstatus.PermissionRequested,
	"Stop":              dstatus.TurnEnded,
	"SessionEnd":        dstatus.SessionEnded,
}

// HookEventNames lists every hook event the listener maps to a Kind, plus
// Notification, whose kind depends on notification_type. hooks.Render takes
// this list, so the settings file and the listener can never drift (issue
// #78). Sorted, so the rendered file is stable.
//
//	content, _ := hooks.Render(bin, status.HookEventNames())
func HookEventNames() []string {
	names := make([]string, 0, len(kindByEvent)+1)
	for name := range kindByEvent {
		names = append(names, name)
	}
	names = append(names, "Notification")
	sort.Strings(names)
	return names
}

// KindOf maps a hook payload to the status event it represents. ok is false
// for events omatty does not track, which the listener drops.
func KindOf(p dstatus.HookPayload) (dstatus.Kind, bool) {
	if p.HookEventName == "Notification" {
		return notificationKind(p.NotificationType)
	}
	if p.HookEventName == "SessionStart" && startsNewConversation(p.Source) {
		return dstatus.SessionRebound, true
	}
	kind, ok := kindByEvent[p.HookEventName]
	return kind, ok
}

// startsNewConversation reports whether a SessionStart's source moved a
// running claude onto a new session id. Only these two: a desktop Claude Code
// forked from a pane inherits its settings and reports a new id in the same
// place with source resume, and re-binding on that would hand the pane to
// someone else's conversation (#316).
func startsNewConversation(source string) bool {
	return source == "clear" || source == "compact"
}

func notificationKind(notifType string) (dstatus.Kind, bool) {
	switch notifType {
	case "idle_prompt":
		return dstatus.Idle, true
	case "permission_prompt":
		return dstatus.PermissionRequested, true
	default:
		return 0, false
	}
}
