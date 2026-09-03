package watcher

import (
	"slices"
	"testing"
)

// Regression, issue #78: the hook event names were listed in hooks and again
// in the listener with nothing binding them; one list is derived from the
// listener's map so an event cannot be registered and then dropped, or
// mapped and never registered.
func TestHookEventNames_MatchesTheListenerMap_issue78(t *testing.T) {
	names := HookEventNames()

	for name := range kindByEvent {
		if !slices.Contains(names, name) {
			t.Errorf("HookEventNames() lacks %q, which the listener maps", name)
		}
	}
	if !slices.Contains(names, "Notification") {
		t.Error("HookEventNames() lacks Notification, which the listener handles by notification_type")
	}
	if len(names) != len(kindByEvent)+1 {
		t.Errorf("HookEventNames() has %d names, want %d", len(names), len(kindByEvent)+1)
	}
	if !slices.IsSorted(names) {
		t.Errorf("HookEventNames() = %v, want sorted for a stable hooks.json", names)
	}
}
