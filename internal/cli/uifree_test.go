package cli_test

import (
	"os/exec"
	"strings"
	"testing"
)

// ADR 0001's test of the whole architecture (#653): the second driving
// adapter builds with no TUI library anywhere beneath it, so the services and
// the domain it reaches are free of the UI. A bubbletea, lipgloss or bubbles
// import anywhere in its graph means a service has reached up into the TUI.
func TestCLI_importsNoUILibrary_issue653(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, out)
	}
	for _, dep := range strings.Fields(string(out)) {
		if strings.HasPrefix(dep, "charm.land/") || strings.Contains(dep, "bubbleterm") {
			t.Errorf("internal/cli's import graph holds %s; the core must be UI-free", dep)
		}
	}
}
