package gate_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/gate"
)

// Nothing here substitutes a fake for exec. The thing under test *is* the
// exec boundary, and #43 is what happens when a package's tests assert the
// command line it builds while nothing ever runs it. Steps are inline shell
// so a reader sees the whole fixture in the assertion.

func TestRun_allStepsPass_reportsEachAsPass(t *testing.T) {
	steps := []gate.Step{
		{Name: "fmt", Run: "true"},
		{Name: "test", Run: "echo ok"},
	}

	got, err := gate.Run(context.Background(), t.TempDir(), steps)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(got))
	}
	for _, r := range got {
		if r.Verdict != gate.Pass {
			t.Errorf("step %q verdict = %v, want Pass", r.Step.Name, r.Verdict)
		}
		if r.ExitCode != 0 {
			t.Errorf("step %q exit = %d, want 0", r.Step.Name, r.ExitCode)
		}
	}
}

// A gate stops at the first failure: the steps after it never ran, and saying
// Pending rather than Pass is the difference the sidebar strip renders.
func TestRun_firstFailureStopsTheRun_laterStepsStayPending(t *testing.T) {
	steps := []gate.Step{
		{Name: "fmt", Run: "true"},
		{Name: "test", Run: "echo boom >&2; exit 3"},
		{Name: "cov", Run: "echo should-not-run"},
	}

	got, _ := gate.Run(context.Background(), t.TempDir(), steps)

	want := []gate.Verdict{gate.Pass, gate.Fail, gate.Pending}
	for i, w := range want {
		if got[i].Verdict != w {
			t.Errorf("step %q verdict = %v, want %v", got[i].Step.Name, got[i].Verdict, w)
		}
	}
	if got[1].ExitCode != 3 {
		t.Errorf("failing step exit = %d, want 3", got[1].ExitCode)
	}
	if !strings.Contains(got[1].Output, "boom") {
		t.Errorf("failing step output = %q, want it to carry stderr", got[1].Output)
	}
}

// Invariant 12. A tool that is not installed is not a broken codebase, and
// sending a session off to fix a lint failure that is really a missing binary
// is the specific waste this prevents. Probed before it was written: under
// sh -c, cmd.Err is nil and the shell returns 127, so the check has to be a
// LookPath pre-flight rather than a Start error.
func TestRun_absentTool_isMissingAndDoesNotRun(t *testing.T) {
	dir := t.TempDir()
	steps := []gate.Step{
		{Name: "lint", Run: "omatty-no-such-tool-xyz --check > ranfile"},
	}

	got, _ := gate.Run(context.Background(), dir, steps)

	if got[0].Verdict != gate.Missing {
		t.Fatalf("verdict = %v, want Missing", got[0].Verdict)
	}
	if !strings.Contains(got[0].Output, "omatty-no-such-tool-xyz") {
		t.Errorf("output = %q, want it to name the tool that is absent", got[0].Output)
	}
	if _, err := readFile(filepath.Join(dir, "ranfile")); err == nil {
		t.Error("the step ran; a Missing step must not be executed at all")
	}
}

// The step's own exit status decides, not the presence of the word FAIL in
// its output (invariant 12).
func TestRun_outputSayingFail_doesNotDecideTheVerdict(t *testing.T) {
	steps := []gate.Step{{Name: "test", Run: "echo 'FAIL github.com/x/y 0.1s'; exit 0"}}

	got, _ := gate.Run(context.Background(), t.TempDir(), steps)

	if got[0].Verdict != gate.Pass {
		t.Errorf("verdict = %v, want Pass: exit 0 is the fact, the text is a rendering", got[0].Verdict)
	}
}

func TestRun_runsEachStepInTheGivenDirectory(t *testing.T) {
	dir := t.TempDir()
	steps := []gate.Step{{Name: "pwd", Run: "pwd > where"}}

	if _, err := gate.Run(context.Background(), dir, steps); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got, err := readFile(filepath.Join(dir, "where"))
	if err != nil {
		t.Fatalf("step did not run in %s: %v", dir, err)
	}
	// macOS resolves TempDir through /private, so compare the resolved pair.
	wantResolved, _ := filepath.EvalSymlinks(dir)
	gotResolved, _ := filepath.EvalSymlinks(strings.TrimSpace(got))
	if gotResolved != wantResolved {
		t.Errorf("step ran in %q, want %q", gotResolved, wantResolved)
	}
}

func TestRun_cancelledContext_stopsTheChildAndReportsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	steps := []gate.Step{{Name: "slow", Run: "sleep 30"}}

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	got, _ := gate.Run(ctx, t.TempDir(), steps)

	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("Run took %v; the child outlived the cancel", elapsed)
	}
	if got[0].Verdict != gate.Cancelled {
		t.Errorf("verdict = %v, want Cancelled", got[0].Verdict)
	}
}

func TestRun_directoryThatIsNotADirectory_isAnErrorNamingIt(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")

	_, err := gate.Run(context.Background(), missing, []gate.Step{{Name: "x", Run: "true"}})

	if err == nil {
		t.Fatal("Run() error = nil, want an error naming the directory")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error = %q, want it to carry the offending path", err)
	}
}

func TestRun_noSteps_isAnEmptyRunNotAnError(t *testing.T) {
	got, err := gate.Run(context.Background(), t.TempDir(), nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("len(results) = %d, want 0", len(got))
	}
}

func TestRun_recordsHowLongAStepTook(t *testing.T) {
	got, _ := gate.Run(context.Background(), t.TempDir(), []gate.Step{{Name: "x", Run: "true"}})

	if got[0].Elapsed <= 0 {
		t.Errorf("Elapsed = %v, want a positive duration", got[0].Elapsed)
	}
}

// A path that exists but is a file is the mistake worth naming separately:
// "not a directory" tells the caller what is wrong, where a stat error does not.
func TestRun_directoryThatIsAFile_saysSo(t *testing.T) {
	file := filepath.Join(t.TempDir(), "a-file")
	if err := writeFile(file, "x"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := gate.Run(context.Background(), file, []gate.Step{{Name: "x", Run: "true"}})

	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("Run() error = %v, want it to say the path is not a directory", err)
	}
}

// Lines leadingWord declines to read are run, not pre-judged. Each of these
// would be a wrongly-skipped step if the pre-flight were less careful.
func TestRun_linesThePreflightCannotRead_areStillRun(t *testing.T) {
	steps := []gate.Step{
		{Name: "env", Run: "CGO_ENABLED=0 true"},
		{Name: "builtin", Run: "cd . && true"},
		{Name: "subshell", Run: "(true)"},
		{Name: "empty", Run: ""},
	}

	got, err := gate.Run(context.Background(), t.TempDir(), steps)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, r := range got {
		if r.Verdict != gate.Pass {
			t.Errorf("step %q verdict = %v, want Pass: the pre-flight must not skip it", r.Step.Name, r.Verdict)
		}
	}
}

// Cancelling must kill the step's whole process tree, not just the sh that
// started it. Found by CI on #224: macOS passed and ubuntu did not, because
// killing the direct child suffices only where sh exec'd the command into
// itself. This step deliberately backgrounds a subshell so sh cannot exec it,
// which is the shape that survived.
//
// The orphan matters more than the delay: a cancelled `go test -race` that
// keeps running is the machine-thrashing #229's parallelism bound exists to
// prevent.
func TestRun_cancel_killsTheWholeProcessGroup_issue224(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "survived")
	ctx, cancel := context.WithCancel(context.Background())
	steps := []gate.Step{{Name: "forks", Run: "(sleep 1; touch survived) & wait"}}

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	got, _ := gate.Run(ctx, dir, steps)

	if got[0].Verdict != gate.Cancelled {
		t.Fatalf("verdict = %v, want Cancelled", got[0].Verdict)
	}
	// Outlive the backgrounded sleep: if anything in the group survived the
	// cancel, it has had its chance to write by now.
	time.Sleep(1500 * time.Millisecond)
	if _, err := readFile(marker); err == nil {
		t.Error("a process in the step's group outlived the cancel and kept working")
	}
}
