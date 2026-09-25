package forge_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// fakeGH writes a gh stand-in that records where and how it was called, then
// prints out and exits with code, writing errOut to stderr. No test here runs
// the real gh or reaches the network.
func fakeGH(t *testing.T, out, errOut string, code int) (bin, calls string) {
	t.Helper()
	dir := t.TempDir()
	calls = filepath.Join(dir, "calls")
	for name, body := range map[string]string{"out": out, "err": errOut} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	script := "#!/bin/sh\n" +
		`printf '%s|%s\n' "$PWD" "$*" >> '` + calls + "'\n" +
		`cat '` + filepath.Join(dir, "out") + "'\n" +
		`cat '` + filepath.Join(dir, "err") + "' >&2\n" +
		"exit " + string(rune('0'+code)) + "\n"
	bin = filepath.Join(dir, "gh")
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return bin, calls
}

// Two calls per project, both in its root, each asking for exactly the
// fields Fold reads - so gh resolves the repository from the checkout's own
// remote. Until #358 this was one `--state all` call; the open set is now
// asked for whole, and the finished window without the checks it never shows.
func TestCLI_ListPRsRunsItsListsInTheRepoRoot_issue310(t *testing.T) {
	bin, calls := ghByState{Answers: map[string]string{
		"open": `[{"number":7,"headRefName":"feat-x","state":"OPEN","mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`,
	}}.install(t)
	root := t.TempDir()

	prs, err := forge.NewCLIWithBin(bin).ListPRs(root)

	if err != nil || len(prs) != 1 || prs[0].Number != 7 || prs[0].Branch != "feat-x" {
		t.Fatalf("ListPRs() = %+v, %v; want #7 on feat-x", prs, err)
	}
	got, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"pr list --state open --limit 100 --json number,headRefName,headRefOid,isCrossRepository,state,mergeStateStatus,statusCheckRollup",
		"pr list --state closed --limit 30 --json number,headRefName,headRefOid,isCrossRepository,state",
	}
	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	if len(lines) != len(want) {
		t.Fatalf("gh was called %d times, want %d:\n%s", len(lines), len(want), got)
	}
	for i, line := range lines {
		dir, args, _ := strings.Cut(line, "|")
		if args != want[i] || !sameDir(t, dir, root) {
			t.Errorf("call %d was %q in %s, want %q in %s", i, args, dir, want[i], root)
		}
	}
}

// A checkout gh cannot map to a GitHub repository is not a failure to
// report: the project simply has no pull requests omatty can read.
func TestCLI_ListPRsRecognisesARepoThatIsNotOnGitHub_issue310(t *testing.T) {
	for _, stderr := range []string{
		"no git remotes found",
		"none of the git remotes configured for this repository point to a known GitHub host. To tell gh about a new GitHub host, please use `gh auth login`",
		"failed to run git: fatal: not a git repository (or any of the parent directories): .git",
	} {
		bin, _ := fakeGH(t, "", stderr, 1)
		_, err := forge.NewCLIWithBin(bin).ListPRs(t.TempDir())
		if !errors.Is(err, forge.ErrNotGitHub) {
			t.Errorf("stderr %q: error = %v, want ErrNotGitHub", stderr, err)
		}
	}
}

// Any other failure - no auth, no network - is an ordinary error carrying
// gh's own words, since they are the only diagnostic anyone can act on.
func TestCLI_ListPRsCarriesAnyOtherFailure_issue310(t *testing.T) {
	bin, _ := fakeGH(t, "", "HTTP 401: Bad credentials", 1)

	_, err := forge.NewCLIWithBin(bin).ListPRs(t.TempDir())

	if err == nil || errors.Is(err, forge.ErrNotGitHub) || errors.Is(err, forge.ErrNoGH) {
		t.Fatalf("error = %v, want an ordinary error", err)
	}
	if !strings.Contains(err.Error(), "Bad credentials") {
		t.Errorf("error = %v, want gh's stderr in it", err)
	}
}

// Without gh there is nothing to ask, and that must read as "not installed",
// never as a failed call: the gate learned it with a missing tool (#248).
func TestCLI_ListPRsWithoutGhIsErrNoGH_issue310(t *testing.T) {
	_, err := forge.NewCLIWithBin(filepath.Join(t.TempDir(), "no-such-gh")).ListPRs(t.TempDir())

	if !errors.Is(err, forge.ErrNoGH) {
		t.Errorf("error = %v, want ErrNoGH", err)
	}
}

// sameDir compares two paths after resolving symlinks: macOS's temp dir is
// /var, a link to /private/var, and a shell's $PWD may report either.
func sameDir(t *testing.T, a, b string) bool {
	t.Helper()
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}
