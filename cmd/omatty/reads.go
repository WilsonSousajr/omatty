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
	// Claude is the only profile today; an empty name resolves to it (#46).
	profile, err := lookupAgent("")
	if err != nil {
		return err
	}
	return cli.Status(ctx, out, store, statusReader(home, profile))
}

// statusReader reads one session's transcript once, through the reader the
// TUI's tailer uses, into the state its card would show.
func statusReader(home string, profile agent.Profile) cli.StatusReader {
	return func(s session.Session) dstatus.SessionState {
		conv := s.ConversationID()
		return status.Read(conv, openTranscript(profile.TranscriptPath(home, s.Dir, conv)), profile.Status, time.Now)
	}
}
