package hooks_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/hooks"
)

// Regression, issue #316: /clear moves claude onto a new session id and says
// so only in SessionStart's source. Parsing dropped the field, so omatty
// could not tell a cleared pane from any other start.
func TestParsePayload_ReadsTheSessionStartSource_issue316(t *testing.T) {
	in := `{"session_id":"new","hook_event_name":"SessionStart","source":"clear","cwd":"/w"}`

	p, ok := hooks.ParsePayload(strings.NewReader(in))

	if !ok || p.Source != "clear" {
		t.Errorf("ParsePayload = (%+v, %v), want source clear", p, ok)
	}
}

// The owning session comes from the hook's own environment, never from what
// claude wrote on stdin: a payload naming a pane must not be able to steer a
// re-bind onto it (#316).
func TestParsePayload_IgnoresAnOmattySessionOnStdin_issue316(t *testing.T) {
	in := `{"session_id":"new","hook_event_name":"SessionStart","omatty_session":"victim"}`

	p, ok := hooks.ParsePayload(strings.NewReader(in))

	if !ok || p.OmattySession != "" {
		t.Errorf("ParsePayload = (%+v, %v), want omatty_session left empty", p, ok)
	}
}

// Regression, issue #316: the hook forwards the registry id the launcher put
// in its environment, which is the only thing that names the pane a cleared
// session belongs to - two panes can share a directory.
func TestReport_ForwardsTheOwningSession_issue316(t *testing.T) {
	path, got := listen(t)
	stdin := strings.NewReader(`{"session_id":"new","hook_event_name":"SessionStart","source":"clear"}`)

	if err := hooks.Report(stdin, path, time.Second, "row-1"); err != nil {
		t.Fatalf("Report() error = %v, want nil", err)
	}

	select {
	case line := <-got:
		var p hooks.Payload
		if err := json.Unmarshal([]byte(line), &p); err != nil || p.OmattySession != "row-1" || p.Source != "clear" {
			t.Errorf("forwarded %q (err %v), want omatty_session row-1 and source clear", line, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listener never received the payload")
	}
}
