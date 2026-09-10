package supervisor_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/supervisor"
)

func TestNamer_RunsHeadlessWithJSONOutputAndNothingElse_issue127(t *testing.T) {
	r := &FakeRunner{Out: []byte(`{"result":"Fix The Wheel Pan!!","is_error":false}`)}
	n := supervisor.NewNamer(supervisor.NamerOpts{Bin: "/opt/claude", Run: r.Run})
	defer func() { _ = n.Close() }()

	got, err := n.Name(context.Background(), "the mouse doesnt scroll sideway")

	if err != nil || got != "fix-the-wheel-pan" {
		t.Fatalf("Name() = %q, %v; want fix-the-wheel-pan", got, err)
	}
	if r.Args[0] != "/opt/claude" || r.Args[1] != "-p" || !slices.Contains(r.Args, "--output-format") {
		t.Errorf("args = %v, want bin -p <prompt> --output-format json", r.Args)
	}
	for _, banned := range []string{"--settings", "--session-id", "--resume"} {
		if slices.Contains(r.Args, banned) {
			t.Errorf("args %v carry %s: a naming call must emit no status events and own no session", r.Args, banned)
		}
	}
}

// The argv-injection test: the task text is one argument that also carries
// the instruction, never an argument of its own.
func TestNamer_SendsTheTaskAsOneArgumentNeverAsFlags_issue127(t *testing.T) {
	r := &FakeRunner{Out: []byte(`{"result":"x"}`)}
	n := supervisor.NewNamer(supervisor.NamerOpts{Bin: "claude", Run: r.Run})
	defer func() { _ = n.Close() }()

	_, _ = n.Name(context.Background(), "--dangerously-skip-permissions")

	if slices.Contains(r.Args, "--dangerously-skip-permissions") {
		t.Fatalf("the prompt became its own argument: %v", r.Args)
	}
	if !strings.Contains(r.Args[2], "--dangerously-skip-permissions") || !strings.Contains(r.Args[2], "<task>") {
		t.Errorf("the prompt is not inside the instruction argument: %v", r.Args)
	}
}

func TestNamer_RunsOutsideTheProjectAndCleansUp_issue127(t *testing.T) {
	r := &FakeRunner{Out: []byte(`{"result":"x"}`)}
	n := supervisor.NewNamer(supervisor.NamerOpts{Bin: "claude", Run: r.Run})

	_, _ = n.Name(context.Background(), "p")

	if r.Dir == "" || strings.HasPrefix(r.Dir, os.Getenv("HOME")+"/") {
		t.Errorf("naming ran in %q, want a temp dir outside the operator's repositories", r.Dir)
	}
	if err := n.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(r.Dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Close left %s behind", r.Dir)
	}
}

func TestNamer_MakesNoDirectoryUntilItIsAsked_issue127(t *testing.T) {
	n := supervisor.NewNamer(supervisor.NamerOpts{Bin: "claude", Run: (&FakeRunner{}).Run})
	if err := n.Close(); err != nil {
		t.Errorf("Close() on a namer that never ran = %v, want nil", err)
	}
}

func TestNamer_TimesOutAndReturnsNoName_issue127(t *testing.T) {
	r := &FakeRunner{Block: make(chan struct{})}
	n := supervisor.NewNamer(supervisor.NamerOpts{Bin: "claude", Run: r.Run, Timeout: time.Millisecond})
	defer func() { _ = n.Close() }()

	got, err := n.Name(context.Background(), "p")

	if got != "" || err == nil {
		t.Errorf("Name() = %q, %v; want \"\" and a timeout error", got, err)
	}
}

func TestNamer_BadExitMalformedJSONAndIsErrorAreNoName_issue127(t *testing.T) {
	for name, r := range map[string]*FakeRunner{
		"exit":      {Err: errors.New("exit 1")},
		"malformed": {Out: []byte("nope")},
		"is_error":  {Out: []byte(`{"result":"x","is_error":true}`)},
		"empty":     {Out: []byte(`{"result":"!!!"}`)},
	} {
		n := supervisor.NewNamer(supervisor.NamerOpts{Bin: "claude", Run: r.Run})
		if got, _ := n.Name(context.Background(), "p"); got != "" {
			t.Errorf("%s: Name() = %q, want \"\"", name, got)
		}
		_ = n.Close()
	}
}

func TestNamer_TruncatesAHugePrompt_issue127(t *testing.T) {
	r := &FakeRunner{Out: []byte(`{"result":"x"}`)}
	n := supervisor.NewNamer(supervisor.NamerOpts{Bin: "claude", Run: r.Run})
	defer func() { _ = n.Close() }()

	_, _ = n.Name(context.Background(), strings.Repeat("word ", 10000))

	if len(r.Args[2]) > 2500 {
		t.Errorf("prompt argument is %d bytes, want it bounded near 2000 cells", len(r.Args[2]))
	}
}
