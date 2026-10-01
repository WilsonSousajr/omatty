package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/cli"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"github.com/WilsonSousajr/omatty/internal/domain/status"
)

func twoSessions() *FakeStore {
	return &FakeStore{State: session.State{Version: session.Version,
		Projects: []session.Project{{Name: "omatty", Root: "/p/omatty"}},
		Sessions: []session.Session{
			{ID: "s1", Project: "omatty", Title: "parser fix", Dir: "/wt/s1", Branch: "parser-fix", Worktree: true},
			{ID: "s2", Project: "omatty", Title: "main", Dir: "/p/omatty", Conversation: "after-clear"},
		}}}
}

// `omatty sessions --json` (ADR 0001, "The second driving adapter"; migration
// step 7.1, #653): every registered session, as state.json holds it, in a
// shape a script can read.
func TestSessions_listsEveryRegisteredSession_issue653(t *testing.T) {
	var out bytes.Buffer
	if err := cli.Sessions(t.Context(), &out, twoSessions()); err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not a JSON array: %v\n%s", err, out.String())
	}
	if len(got) != 2 || got[0]["id"] != "s1" || got[0]["worktree"] != true || got[1]["conversation"] != "after-clear" {
		t.Errorf("sessions = %v", got)
	}
}

// `omatty status --json`: each session's status as its transcript implies it,
// read once. A session with no transcript yet reads idle, as its card does.
func TestStatus_readsEachSessionsTranscript_issue653(t *testing.T) {
	at := time.Date(2026, 9, 2, 12, 0, 5, 0, time.UTC)
	read := func(s session.Session) status.SessionState {
		if s.ID == "s1" {
			return status.SessionState{Status: status.StatusWaiting, At: at, Tokens: status.Tokens{In: 10, Out: 5}}
		}
		return status.SessionState{}
	}
	var out bytes.Buffer
	if err := cli.Status(t.Context(), &out, twoSessions(), read); err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not a JSON array: %v\n%s", err, out.String())
	}
	if tokens, _ := got[0]["tokens"].(map[string]any); tokens["input"] != 10.0 || tokens["output"] != 5.0 {
		t.Errorf("s1 tokens = %v, want input 10 and output 5", got[0]["tokens"])
	}
	if got[0]["status"] != "waiting" || got[0]["since"] != "2026-09-02T12:00:05Z" {
		t.Errorf("s1 = %v, want waiting since 12:00:05", got[0])
	}
	if got[1]["status"] != "idle" || got[1]["since"] != nil {
		t.Errorf("s2 = %v, want idle with no since", got[1])
	}
}

// A state.json that will not load is an error naming it, never an empty list
// a script would read as "no sessions".
func TestSessions_aStoreThatWillNotLoadIsAnError_issue653(t *testing.T) {
	err := cli.Sessions(t.Context(), &bytes.Buffer{}, &FakeStore{Err: errors.New("state.json is not JSON")})
	if err == nil || !strings.Contains(err.Error(), "not JSON") {
		t.Errorf("err = %v, want the load's failure", err)
	}
}
