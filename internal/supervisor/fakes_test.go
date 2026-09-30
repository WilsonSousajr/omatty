package supervisor_test

import (
	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/infra/hooks"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// claudeProfile stands in for cmd's catalog entry, which this package cannot
// import (ADR 0001, migration step 5.2b, #653): claude's own command template
// and adapter, with the transcript slugged as given. Resolving a symlinked
// directory is the catalog's job, pinned in cmd/omatty (#564).
func claudeProfile() agent.Profile {
	return agent.Profile{Name: "claude", DefaultBin: "claude", Command: agent.ClaudeCommand,
		TranscriptPath: paths.Transcript, HookEvents: status.HookEventNames,
		RenderSettings: hooks.Render, Status: status.ClaudeAdapter()}
}

// fakeHolder stands in for detach.Holder, recording what the launcher asked it
// to wrap and answering with a command the test can recognise. A named type,
// per AGENTS.md, so a failure message says what stood in for dtach.
type fakeHolder struct {
	// Wrapped is the command line Wrap hands back, so a test can tell the
	// launcher's own command from the holder's.
	Wrapped []string
	// WrapErr makes Wrap fail, standing in for an unusable socket path.
	WrapErr error
	// GotID and GotArgs are what Wrap was called with.
	GotID   string
	GotArgs []string
	// Stopped records every session Stop was asked to end.
	Stopped []string
	// HeldIDs are the sessions Held answers true for (#191).
	HeldIDs map[string]bool
}

func (f *fakeHolder) Held(sessionID string) (bool, error) { return f.HeldIDs[sessionID], nil }

func (f *fakeHolder) Wrap(sessionID string, argv []string) ([]string, error) {
	f.GotID, f.GotArgs = sessionID, argv
	if f.WrapErr != nil {
		return nil, f.WrapErr
	}
	if f.Wrapped == nil {
		return argv, nil
	}
	return f.Wrapped, nil
}

func (f *fakeHolder) Stop(sessionID string) error {
	f.Stopped = append(f.Stopped, sessionID)
	return nil
}

func (f *fakeHolder) Persists() bool { return true }

func (f *fakeHolder) Notice() string { return "" }
