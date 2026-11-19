package app_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/tui/app"
)

// footerLine is the frame's last row, colours stripped.
func footerLine(t *testing.T, view string) string {
	t.Helper()
	rows := strings.Split(stripSGR(view), "\n")
	return rows[len(rows)-1]
}

// A key after the leader that is no command says so, rather than looking
// like a command that ran and showed nothing (#751). The leader decided the
// key was omatty's, so it still never reaches the session (invariant 1), and
// the next key clears the notice as every notice clears.
func TestLeader_anUnknownKeySaysSo_issue751(t *testing.T) {
	m, fakes := modelWithFakes(t)
	before := len(fakes["s1"].Msgs)

	leader(m, key('w'))

	if footer := footerLine(t, m.View().Content); !strings.Contains(footer, "ctrl+o w: no such command") {
		t.Errorf("the footer does not say ctrl+o w is no command: %q", footer)
	}
	if got := len(fakes["s1"].Msgs); got != before {
		t.Errorf("the unknown key reached the session: %d messages, want %d", got, before)
	}
	press(m, key('x'))
	if footer := footerLine(t, m.View().Content); strings.Contains(footer, "no such command") {
		t.Errorf("the next key did not clear the notice: %q", footer)
	}
}

// A command that ran raises no such notice (#751).
func TestLeader_aKnownKeySaysNothing_issue751(t *testing.T) {
	m, _ := modelWithFakes(t)

	leader(m, key('d'))

	if footer := footerLine(t, m.View().Content); strings.Contains(footer, "no such command") {
		t.Errorf("ctrl+o d is a command, and the footer says it is not: %q", footer)
	}
}

// esc after the leader is backing out of it, not a mistyped command: it is
// let go quietly (#751).
func TestLeader_escBacksOutQuietly_issue751(t *testing.T) {
	m, _ := modelWithFakes(t)

	leader(m, special(27))

	if footer := footerLine(t, m.View().Content); strings.Contains(footer, "no such command") {
		t.Errorf("esc after the leader is reported as a command: %q", footer)
	}
}

// The leader's table and its dispatch cannot drift (#750): every key a person
// can press after the leader is either in the table and runs, or is not and
// says so (#751). A key handled without a help line passes the dispatch but
// not the table; a help line for a key nothing handles passes the table but
// raises the notice.
func TestLeader_everyKeyIsInTheTableOrSaysItIsNot_issue750(t *testing.T) {
	for _, k := range allKeys() {
		switch k.Keystroke() {
		case "esc", "ctrl+o", "ctrl+c":
			// Backing out (#751), the leader itself (#747), and the escape
			// hatch every surface takes before any table (#28, #749).
			continue
		}
		m, _ := modelWithFakes(t)
		press(m, ctrl('o'))
		m.Update(k)
		unknown := strings.Contains(footerLine(t, m.View().Content), "no such command")
		if owned := app.LeaderOwns(k.Keystroke()); owned == unknown {
			t.Errorf("ctrl+o %s: in the leader's table = %v, but says it is no command = %v", k.Keystroke(), owned, unknown)
		}
	}
}
