package cli_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/cli"
)

// `omatty --resume <id>` is what claude's exit hint names once omatty sets
// CLAUDE_CODE_RESUME_HINT_COMMAND (#815): the id is a conversation, and the
// answer is the row that owns it.
func TestResumeTarget_namesTheRowThatOwnsTheConversation_issue815(t *testing.T) {
	got, err := cli.ResumeTarget(twoSessions().State, []string{"after-clear"})

	if err != nil || got != "s2" {
		t.Errorf("ResumeTarget(after-clear) = %q, %v; want s2, nil", got, err)
	}
}

// A conversation no row owns is said with the id and what to do instead,
// not opened as an empty omatty the person has to search.
func TestResumeTarget_refusesAConversationNoRowOwns_issue815(t *testing.T) {
	_, err := cli.ResumeTarget(twoSessions().State, []string{"elsewhere"})

	if err == nil || !strings.Contains(err.Error(), `"elsewhere"`) || !strings.Contains(err.Error(), "omatty adopt") {
		t.Errorf("ResumeTarget(elsewhere) error = %v; want the id and `omatty adopt`", err)
	}
}

// claude prints exactly one id after the flag; anything else is a mistake
// worth naming rather than a guess.
func TestResumeTarget_wantsExactlyOneID_issue815(t *testing.T) {
	for _, args := range [][]string{nil, {"a", "b"}, {""}} {
		if _, err := cli.ResumeTarget(twoSessions().State, args); err == nil {
			t.Errorf("ResumeTarget(%q) error = nil; want a usage error", args)
		}
	}
}
