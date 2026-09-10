// Naming a session by asking the agent's own binary, headless (#127 step 2).
//
// It lives in supervisor because supervisor is the package that runs the
// claude binary - invariant 4's argument, applied to a second way of running
// it. Three things it deliberately does not do. It does not go through the
// detach holder: a naming call is a one-shot that must die with its timeout,
// and a held one would outlive omatty with nothing attached to it. It does
// not pass --settings: the hooks file is what makes claude report status
// events, and a naming call is not a session anyone is watching. And it does
// not pass --session-id: omatty assigns uuids to sessions it will resume, and
// this is not one.

package supervisor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

// Runner executes the naming call. Injected so a test can assert the exact
// argument list without a claude on the machine.
type Runner func(ctx context.Context, dir string, args []string) ([]byte, error)

// NamerOpts configures a Namer. Timeout and Run default when zero, the way
// ui.Deps fills its optional fields.
type NamerOpts struct {
	Bin     string
	Timeout time.Duration
	Run     Runner
}

// defaultNamingTimeout bounds one headless call: long enough for a small
// model round trip, short enough that a stalled one is invisible.
const defaultNamingTimeout = 10 * time.Second

// maxPromptCells bounds what is sent. A first prompt can be a pasted essay
// and the name is decided by its first sentence.
const maxPromptCells = 2000

// namingModel is the model asked. Naming needs no intelligence, and the
// call's cost is dominated by the CLI's own system prompt, so the smallest
// model is the right one.
const namingModel = "haiku"

// Namer asks the agent's binary for a short name for a task.
//
//	n := supervisor.NewNamer(supervisor.NamerOpts{Bin: cfg.ClaudeBin})
//	defer n.Close()
//	name, err := n.Name(ctx, "the mouse doesnt scroll sideway in the diff")
type Namer struct {
	bin     string
	timeout time.Duration
	run     Runner
	dir     string // working directory, made on first use
}

// NewNamer returns a Namer for bin.
func NewNamer(o NamerOpts) *Namer {
	if o.Timeout == 0 {
		o.Timeout = defaultNamingTimeout
	}
	if o.Run == nil {
		o.Run = execRunner
	}
	return &Namer{bin: o.Bin, timeout: o.Timeout, run: o.Run}
}

// Name returns a slug for prompt, or "" when the call failed, timed out, or
// produced nothing usable. The error is for the log; the caller keeps the
// name it already had either way, because naming must never be something the
// operator waits on (#127).
func (n *Namer) Name(ctx context.Context, prompt string) (string, error) {
	dir, err := n.workDir()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, n.timeout)
	defer cancel()
	args := []string{n.bin, "-p", namingPrompt(prompt), "--output-format", "json", "--model", namingModel}
	out, err := n.run(ctx, dir, args)
	if err != nil {
		return "", fmt.Errorf("naming call %s: %w", n.bin, err)
	}
	return parseName(out)
}

// namingPrompt wraps an untrusted first prompt in the instruction that turns
// it into a name. The task text goes last and inside a tag, and the whole
// thing is one argv element: it is data being described, never a flag and
// never a shell word (AGENTS.md, Security).
func namingPrompt(prompt string) string {
	return "Reply with only a 2-5 word lowercase hyphenated name for this task, nothing else.\n<task>\n" +
		runewidth.Truncate(prompt, maxPromptCells, "") + "\n</task>"
}

// result is `claude -p --output-format json`, parsed into a struct at the
// edge, once.
type result struct {
	Result  string `json:"result"`
	IsError bool   `json:"is_error"`
}

func parseName(out []byte) (string, error) {
	var r result
	if err := json.Unmarshal(out, &r); err != nil {
		return "", fmt.Errorf("naming call: output %q is not the json shape {result, is_error}: %w", clipBytes(out), err)
	}
	if r.IsError {
		return "", fmt.Errorf("naming call: claude reported an error: %q", clipBytes([]byte(r.Result)))
	}
	return registry.Slug(r.Result), nil
}

// clipBytes bounds what an error message quotes.
func clipBytes(b []byte) string {
	const keep = 80
	if len(b) > keep {
		return string(b[:keep]) + "..."
	}
	return string(b)
}

// workDir is where naming calls run: a temp directory outside every
// repository, made once per omatty run. claude writes a transcript for every
// headless call under a slug named after the working directory, so a
// per-call directory would leave one junk slug per session ever created; and
// running under ~/.omatty would - if the operator's home is itself a git
// repository - make discovery offer their home as a project (#91, #122,
// #127). A directory that no longer exists is what discover.candidateOf
// already filters out.
func (n *Namer) workDir() (string, error) {
	if n.dir != "" {
		return n.dir, nil
	}
	dir, err := os.MkdirTemp("", "omatty-naming-")
	if err != nil {
		return "", fmt.Errorf("naming call: creating a working directory: %w", err)
	}
	n.dir = dir
	return dir, nil
}

// Close removes the working directory, if one was made.
func (n *Namer) Close() error {
	if n.dir == "" {
		return nil
	}
	dir := n.dir
	n.dir = ""
	return os.RemoveAll(dir)
}

// execRunner is the real Runner. CommandContext kills the process on
// timeout, and WaitDelay lets a grandchild holding the pipes not outlive it.
// Stderr goes to a bounded buffer for the error, never to the terminal
// (invariant 5); stdin is nil, which claude reads as an immediate EOF rather
// than waiting three seconds for piped input.
func execRunner(ctx context.Context, dir string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	cmd.WaitDelay = time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, clipBytes(stderr.Bytes()))
	}
	return out, nil
}
