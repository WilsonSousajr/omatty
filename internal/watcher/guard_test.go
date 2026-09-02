package watcher

import "testing"

// Invariant 6: a panic in a watcher goroutine must be swallowed, not fatal.
func TestRecoverLoop_SwallowsAPanic_issue65(_ *testing.T) {
	func() {
		defer recoverLoop("test", "s1")
		panic("boom")
	}()
	// Reaching this line is the assertion.
}
