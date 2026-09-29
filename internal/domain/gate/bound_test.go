package gate_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/gate"
)

// Tail moved here from internal/gate with the rest of the pure output caps
// (migration step 3.3); its callers there test it through Run, so it is
// pinned in its own package too.
func TestTail_keepsTheEndAndSaysWhatItDropped_issue635(t *testing.T) {
	out := strings.Repeat("line\n", gate.MaxOutputLines+5)

	got := gate.Tail(out, 0)

	if !strings.HasPrefix(got, "… 5 earlier lines elided …\n") {
		t.Errorf("Tail did not say it dropped 5 lines: %q", got[:40])
	}
}

func TestTail_countsWhatTheCollectingWindowAlreadyDropped_issue635(t *testing.T) {
	got := gate.Tail("short\n", 100)

	if !strings.HasPrefix(got, "… 100 earlier bytes elided …\n") {
		t.Errorf("Tail lost the window's drop count: %q", got)
	}
}

func TestTail_outputUnderTheCapsIsWhole_issue635(t *testing.T) {
	if got := gate.Tail("ok\n", 0); got != "ok\n" {
		t.Errorf("Tail(%q) = %q, want it unchanged", "ok\n", got)
	}
}
