package forge_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// callsIn reads the fake gh's log.
func callsIn(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the fake gh was never called: %v", err)
	}
	return string(body)
}

// #331's step 1: push, then open the pull request. This is omatty's first write
// to the forge, and it goes through the same bounded runner as every read.
func TestCreatePR_OpensThePullRequestAndReturnsItsNumber_issue331(t *testing.T) {
	bin, calls := fakeGH(t, "https://github.com/o/r/pull/443\n", "", 0)
	root := t.TempDir()

	number, err := forge.NewCLIWithBin(bin).CreatePR(root, "feat/a", "develop", "feat(#1): a thing")
	if err != nil {
		t.Fatal(err)
	}

	if number != 443 {
		t.Errorf("number = %d, want 443 read from the url gh prints", number)
	}
	got := callsIn(t, calls)
	for _, want := range []string{"pr create", "--head feat/a", "--base develop", root} {
		if !strings.Contains(got, want) {
			t.Errorf("%q missing from the call: %q", want, got)
		}
	}
}

// gh refusing is the operator's answer, carried whole: a pull request that
// already exists, a branch that was never pushed, a repository with no write
// access. omatty has nothing useful to add to any of them.
func TestCreatePR_CarriesWhatGhSaid_issue331(t *testing.T) {
	bin, _ := fakeGH(t, "", "a pull request for branch \"feat/a\" already exists", 1)

	_, err := forge.NewCLIWithBin(bin).CreatePR(t.TempDir(), "feat/a", "develop", "t")

	if err == nil {
		t.Fatal("gh exiting non-zero was swallowed")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("err = %v, want gh's own reason in it", err)
	}
}

// #331's step 2. No merge method is passed: the repository's own setting is the
// operator's, and a repository allowing several makes gh say so rather than
// omatty choosing one on their behalf.
func TestMergePR_MergesAndNamesNoMethod_issue331(t *testing.T) {
	bin, calls := fakeGH(t, "", "", 0)

	if err := forge.NewCLIWithBin(bin).MergePR(t.TempDir(), 443); err != nil {
		t.Fatal(err)
	}

	got := callsIn(t, calls)
	if !strings.Contains(got, "pr merge 443") {
		t.Errorf("call = %q, want a merge of 443", got)
	}
	for _, never := range []string{"--squash", "--rebase", "--admin", "--delete-branch"} {
		if strings.Contains(got, never) {
			t.Errorf("the merge passed %s, which is not omatty's to choose: %q", never, got)
		}
	}
}

// The bound #331 adds that the issue itself does not: never into a protected
// branch. AGENTS.md moves `main` only by a promotion pull request, so a ship key
// able to merge there would route around omatty's own release gate.
func TestBranchProtected_ReadsTheBranchesOwnFlag_issue331(t *testing.T) {
	bin, calls := fakeGH(t, "true\n", "", 0)

	protected, err := forge.NewCLIWithBin(bin).BranchProtected(t.TempDir(), "main")
	if err != nil {
		t.Fatal(err)
	}

	if !protected {
		t.Error("a branch gh reports as protected came back unprotected")
	}
	if got := callsIn(t, calls); !strings.Contains(got, "main") {
		t.Errorf("call = %q, want it to ask about main", got)
	}
}

// An unprotected branch is the ordinary case and must be quiet.
func TestBranchProtected_FalseForAnOrdinaryBranch_issue331(t *testing.T) {
	bin, _ := fakeGH(t, "false\n", "", 0)

	protected, err := forge.NewCLIWithBin(bin).BranchProtected(t.TempDir(), "develop")
	if err != nil || protected {
		t.Errorf("BranchProtected = %v, %v; want false, nil", protected, err)
	}
}

// It fails closed. A protection flag omatty could not read is not permission to
// merge: the whole point of the check is the one case where being wrong cannot
// be undone.
func TestBranchProtected_FailsClosed_issue331(t *testing.T) {
	bin, _ := fakeGH(t, "", "HTTP 404: Not Found", 1)

	protected, err := forge.NewCLIWithBin(bin).BranchProtected(t.TempDir(), "develop")

	if err == nil {
		t.Fatal("an unreadable protection flag was treated as an answer")
	}
	if !protected {
		t.Error("BranchProtected returned false alongside its error; a caller that " +
			"checks the bool first would merge into a branch it could not read")
	}
}

// gh missing is answered before any process starts, as every read already does.
func TestShipCalls_SayWhenGhIsMissing_issue331(t *testing.T) {
	cli := forge.NewCLIWithBin("gh-that-is-not-installed-anywhere")

	if _, err := cli.CreatePR(t.TempDir(), "feat/a", "develop", "t"); !errors.Is(err, forge.ErrNoGH) {
		t.Errorf("CreatePR err = %v, want ErrNoGH", err)
	}
	if err := cli.MergePR(t.TempDir(), 1); !errors.Is(err, forge.ErrNoGH) {
		t.Errorf("MergePR err = %v, want ErrNoGH", err)
	}
	if _, err := cli.BranchProtected(t.TempDir(), "main"); !errors.Is(err, forge.ErrNoGH) {
		t.Errorf("BranchProtected err = %v, want ErrNoGH", err)
	}
}
