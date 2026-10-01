package status

import (
	"testing"
	"time"

	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
)

// Invariant 11 at the second hop (step 5.2d, #653): the hook server already
// drops rather than wait, and the follower must too. With the event channel
// full and nobody reading, handing it a hook event returns; blocking here
// would back the server's sink up and stall every hook on the machine.
func TestWatch_aHookEventIsDroppedNotWaitedOnWhenEventsAreFull_issue653(t *testing.T) {
	w := &Watch{
		deps:   WatchDeps{Agents: AgentsOf(ClaudeAdapter(), func(_, _, _ string) string { return "" }), Clock: time.Now},
		events: make(chan dstatus.Event), // unbuffered, never read: always full
	}
	done := make(chan struct{})
	go func() {
		w.offerHook(dstatus.HookPayload{SessionID: "s1", HookEventName: "Stop"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the hook follower waited on a full event channel")
	}
}
