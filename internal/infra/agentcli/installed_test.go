package agentcli_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/agentcli"
)

// ctrl+o n offers only an agent whose binary resolves on PATH (#524).
func TestInstalled_AnswersFromPATH_issue524(t *testing.T) {
	if !agentcli.Installed("sh") {
		t.Error("Installed(sh) = false, want true")
	}
	if agentcli.Installed("/nonexistent/omatty-no-such-agent") {
		t.Error("Installed of a missing binary = true, want false")
	}
	if agentcli.Installed("") {
		t.Error(`Installed("") = true, want false`)
	}
}
