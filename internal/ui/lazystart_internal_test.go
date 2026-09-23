package ui

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

func threeSessions() registry.State {
	return registry.State{Sessions: []registry.Session{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}}}
}

// Regression, issue #317: boot spawned a claude for every row - 11 of them,
// 2.99 GB, ten idle for days. Under lazy start it attaches exactly the ones a
// holder already keeps alive, which cost nothing extra, and none other.
func TestSessionsToStart_LazyStartsOnlyTheHeld_issue317(t *testing.T) {
	d := RunDeps{State: threeSessions(), LazyStart: true}

	if got := sessionsToStart(d, map[string]bool{}); len(got) != 0 {
		t.Errorf("lazy start with nothing held starts %v, want none", got)
	}
	if got := sessionsToStart(d, map[string]bool{"s2": true}); len(got) != 1 || !got["s2"] {
		t.Errorf("lazy start with s2 held starts %v, want only s2", got)
	}
}

// lazy_start = false keeps the old boot: every session starts.
func TestSessionsToStart_EagerStartsEverySession_issue317(t *testing.T) {
	d := RunDeps{State: threeSessions(), LazyStart: false}

	if got := sessionsToStart(d, map[string]bool{"s2": true}); len(got) != 3 {
		t.Errorf("eager start starts %v, want all three", got)
	}
}
