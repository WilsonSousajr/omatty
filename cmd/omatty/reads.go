// `omatty sessions --json` and `omatty status --json`: the second driving
// adapter's two reads (ADR 0001; migration step 7.1, #653). cmd builds what
// they read through and internal/cli writes them.

package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/WilsonSousajr/omatty/internal/cli"
	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// readCommand runs one of the two reads. JSON is the only format they print,
// so the flag is required rather than implied: a script that names it keeps
// working if a human-readable form is added later.
func readCommand(cmd string, args []string, home string, store sessions.StateStore, out io.Writer) error {
	if len(args) != 1 || args[0] != "--json" {
		return fmt.Errorf("%s: want --json, got %q (JSON is the only format it prints)", cmd, args)
	}
	ctx := context.Background()
	if cmd == "sessions" {
		return cli.Sessions(ctx, out, store)
	}
	agents, err := agentCatalog()
	if err != nil {
		return err
	}
	return cli.Status(ctx, out, store, statusReader(home, agents))
}

// statusReader reads one session's transcript once, through the reader the
// TUI's tailer uses, into the state its card would show. A session whose
// agent this omatty does not know reads as the zero state, as the TUI shows a
// session it does not tail (#521).
func statusReader(home string, agents agent.Catalog) cli.StatusReader {
	return func(s session.Session) dstatus.SessionState {
		profile, err := agents.Lookup(s.Agent)
		if err != nil || !profile.KeepsTranscript() {
			return dstatus.SessionState{}
		}
		conv := s.ConversationID()
		return status.Read(conv, openTranscript(profile.TranscriptPath(home, s.Dir, conv)), profile.Status, time.Now)
	}
}
