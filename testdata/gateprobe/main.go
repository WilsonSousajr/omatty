// Command gateprobe proves the things M9's unit tests cannot: that a real
// gate, made of real commands, runs through internal/gate against a real
// working directory - and that the Runner's bound, its supersede and its
// LookPath pre-flight behave against the tools this machine actually has.
//
//	go run ./testdata/gateprobe
//	go run ./testdata/gateprobe /path/to/a/checkout
//
// It is the same argument as dtachprobe. internal/gate's tests assert what a
// step's verdict is; they cannot assert that the wiring between Run, the
// Runner and a real directory holds, and #43 is what shipped green the last
// time a package's tests substituted a fake for the thing they were testing.
//
// Not part of the gate; a person reads the output (roadmap rule 2).
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/WilsonSousajr/omatty/internal/gate"
)

// scratch is a checkout whose gate fails in the middle, which is the case
// worth watching: the steps after a failure must report Pending, not Pass.
var scratch = []gate.Step{
	{Name: "fmt", Run: "test -f go.mod"},
	{Name: "test", Run: "echo '--- FAIL: TestThing' >&2; echo 'thing_test.go:12: got 1, want 2' >&2; exit 1"},
	{Name: "cov", Run: "echo 'coverage 92.6% meets the 90% gate'", Kind: gate.KindCoverage},
}

// absent names a tool no machine has, so the LookPath pre-flight is exercised
// against a real PATH rather than a fake one.
var absent = []gate.Step{{Name: "lint", Run: "omatty-no-such-linter run"}}

func main() {
	dir := scratchDir()
	fmt.Println("working directory:", dir)

	fmt.Println("\n--- a gate that fails in the middle ---")
	report(gate.Run(context.Background(), dir, scratch))

	fmt.Println("\n--- a tool that is not installed (invariant 12) ---")
	report(gate.Run(context.Background(), dir, absent))

	fmt.Println("\n--- the Runner: two sessions, bound of 1 ---")
	runner(dir)

	fmt.Println("\n--- superseding a run in flight ---")
	supersede(dir)

	fmt.Println("\n--- what omatty would propose for this checkout ---")
	proposed(".")
}

// proposed prints the gate Detect offers for a real repository, which is the
// half of the loop the scratch steps above cannot show: that omatty reads a
// checkout it did not construct and comes back with the line that checkout
// actually uses. It prints and does not run - running omatty's own gate takes
// minutes, and the point here is the proposal.
func proposed(root string) {
	steps := gate.Detect(root)
	if len(steps) == 0 {
		fmt.Println("  nothing recognised in", root)
		return
	}
	for _, s := range steps {
		kind := ""
		if s.Kind != "" {
			kind = "   [" + s.Kind + "]"
		}
		fmt.Printf("  %-5s $ %s%s\n", s.Name, s.Run, kind)
	}
}

// scratchDir is the directory the probe gates: the argument if given, else a
// temporary checkout holding the one file scratch's first step looks for.
func scratchDir() string {
	if len(os.Args) > 1 {
		return os.Args[1]
	}
	dir, err := os.MkdirTemp("", "gateprobe-")
	if err != nil {
		exit("temp dir: " + err.Error())
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module probe\n"), 0o600); err != nil {
		exit("writing go.mod: " + err.Error())
	}
	return dir
}

// runner starts two sessions against a bound of one and prints when each
// report lands, so a reader can see the second wait for the first.
//
// Which of the two goes first is up to the Go scheduler, not the order they
// were started in: the bound is a semaphore, not a queue. A gate has no reason
// to be FIFO - what matters is that the two do not overlap, which is what the
// one-second gap in the output shows.
func runner(dir string) {
	r := gate.NewRunner(1)
	defer r.Close()
	started := time.Now()
	r.Start("session-a", dir, []gate.Step{{Name: "slow", Run: "sleep 1; true"}})
	r.Start("session-b", dir, []gate.Step{{Name: "slow", Run: "sleep 1; true"}})
	for range 2 {
		rep := <-r.Reports()
		fmt.Printf("  %s reported at %+.1fs\n", rep.ID, time.Since(started).Seconds())
	}
}

// supersede re-gates a session mid-run. Exactly one report must arrive, and it
// must be the new one: a stale verdict would describe code that has changed.
func supersede(dir string) {
	r := gate.NewRunner(2)
	defer r.Close()
	r.Start("session-a", dir, []gate.Step{{Name: "slow", Run: "sleep 5"}})
	r.Start("session-a", dir, []gate.Step{{Name: "quick", Run: "true"}})

	rep := <-r.Reports()
	fmt.Printf("  reported: %s (%s)\n", rep.Results[0].Step.Name, rep.Results[0].Verdict)
	select {
	case extra := <-r.Reports():
		fmt.Printf("  WRONG: a superseded run also reported: %+v\n", extra)
	case <-time.After(500 * time.Millisecond):
		fmt.Println("  and nothing from the superseded run, as it should be")
	}
}

// report prints a run the way a person reads it.
func report(results []gate.StepResult, err error) {
	if err != nil {
		fmt.Println("  error:", err)
		return
	}
	for _, r := range results {
		line := fmt.Sprintf("  %-5s %-9s exit=%d", r.Step.Name, r.Verdict, r.ExitCode)
		if r.Percent > 0 {
			line += fmt.Sprintf(" %.1f%%", r.Percent)
		}
		fmt.Println(line)
		for _, l := range strings.Split(strings.TrimRight(r.Output, "\n"), "\n") {
			if l != "" {
				fmt.Println("        |", l)
			}
		}
	}
}

func exit(msg string) {
	fmt.Fprintln(os.Stderr, "gateprobe:", msg)
	os.Exit(1)
}
