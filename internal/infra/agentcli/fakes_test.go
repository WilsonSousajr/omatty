package agentcli_test

import "context"

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
