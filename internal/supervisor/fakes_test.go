package supervisor_test

import (
	"context"
	"os/exec"
)

// fakeHolder stands in for detach.Holder, recording what the launcher asked it
// to wrap and answering with a command the test can recognise. A named type,
// per AGENTS.md, so a failure message says what stood in for dtach.
type fakeHolder struct {
	// Wrapped is the command Wrap hands back, so a test can tell the launcher's
	// own command from the holder's.
	Wrapped *exec.Cmd
	// WrapErr makes Wrap fail, standing in for an unusable socket path.
	WrapErr error
	// GotID and GotArgs are what Wrap was called with.
	GotID   string
	GotArgs []string
	// Stopped records every session Stop was asked to end.
	Stopped []string
}

func (f *fakeHolder) Wrap(sessionID string, cmd *exec.Cmd) (*exec.Cmd, error) {
	f.GotID, f.GotArgs = sessionID, cmd.Args
	if f.WrapErr != nil {
		return nil, f.WrapErr
	}
	if f.Wrapped == nil {
		return cmd, nil
	}
	return f.Wrapped, nil
}

func (f *fakeHolder) Stop(sessionID string) error {
	f.Stopped = append(f.Stopped, sessionID)
	return nil
}

func (f *fakeHolder) Persists() bool { return true }

func (f *fakeHolder) Notice() string { return "" }

// FakeRunner stands in for the process that would run claude headless,
// recording the argument list and the working directory it was given (#127).
type FakeRunner struct {
	Out   []byte
	Err   error
	Args  []string
	Dir   string
	Block chan struct{} // when non-nil, Run waits on it or on ctx: the timeout test
}

func (f *FakeRunner) Run(ctx context.Context, dir string, args []string) ([]byte, error) {
	f.Args, f.Dir = args, dir
	if f.Block != nil {
		select {
		case <-f.Block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return f.Out, f.Err
}
