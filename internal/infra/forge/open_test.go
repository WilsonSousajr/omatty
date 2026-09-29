package forge

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// macOS opens with open, everything else with xdg-open.
func TestBrowserOpener_IsThePlatformsOwn_issue462(t *testing.T) {
	if browserOpener("darwin") != "open" || browserOpener("linux") != "xdg-open" {
		t.Errorf("openers = %q, %q", browserOpener("darwin"), browserOpener("linux"))
	}
}

// launch returns at once, even when the opener runs on - an xdg-open that
// became the browser - and says so when the opener is not there at all.
func TestLaunch_ReturnsWithoutWaitingForTheOpener_issue462(t *testing.T) {
	dir := t.TempDir()
	slow := filepath.Join(dir, "slow-opener")
	if err := os.WriteFile(slow, []byte("#!/bin/sh\nsleep 5\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	start := time.Now()

	if err := launch(slow, "https://example.test/o/r/issues/1"); err != nil || time.Since(start) > 2*time.Second {
		t.Errorf("launch = %v after %v, want nil at once", err, time.Since(start))
	}
	if err := launch(filepath.Join(dir, "no-such-opener"), "https://example.test"); err == nil {
		t.Error("launch of a missing opener = nil, want an error")
	}
}
