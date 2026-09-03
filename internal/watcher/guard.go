package watcher

import "log/slog"

// recoverLoop is deferred at the top of every long-lived watcher goroutine.
// Invariant 6: one session's panic must not take the app down. termwrap has
// the same guard for the emulator side; the status side had none (issue #65).
// The session's status simply goes stale.
func recoverLoop(role, sessionID string) {
	if r := recover(); r != nil {
		slog.Error("watcher goroutine panicked; its status is now stale",
			"role", role, "session", sessionID, "panic", r)
	}
}
