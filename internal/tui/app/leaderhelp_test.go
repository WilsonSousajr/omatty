package app_test

import (
	"strings"
	"testing"
)

// Issue #747: the passthrough is documented in help, under the leader that is
// actually configured.
func TestHelp_listsTheLeaderPassthrough_issue747(t *testing.T) {
	m, _ := modelWithFakes(t)
	leader(m, key('?'))
	press(m, key('/'))
	for _, r := range "itself" {
		press(m, key(r))
	}

	got := m.View().Content

	if !strings.Contains(got, "ctrl+o ctrl+o") || !strings.Contains(got, "send ctrl+o itself") {
		t.Errorf("help does not document ctrl+o ctrl+o:\n%s", got)
	}
}
