package forge_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// fakeGlab is a glab whose `api` answers each path from the recorded GitLab
// fixtures, as the real one answers from the forge, recording each call. A
// path it has no fixture for fails as glab does on a 404.
func fakeGlab(t *testing.T) (bin, calls string) { return fakeGlabWith(t, nil) }

// glabRoute answers a path pattern with a file, ahead of the fixtures.
type glabRoute struct{ pattern, file string }

// fakeGlabWith is fakeGlab with extra routes tried first.
func fakeGlabWith(t *testing.T, extra []glabRoute) (bin, calls string) {
	t.Helper()
	dir := t.TempDir()
	calls = filepath.Join(dir, "calls")
	fx := func(name string) string { return filepath.Join("..", "..", "testdata", "forge", "gitlab", name) }
	abs := func(name string) string {
		p, err := filepath.Abs(fx(name))
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	script := "#!/bin/sh\n" +
		`printf '%s\n' "$*" >> '` + calls + "'\n" +
		`case "$*" in` + "\n" + extraCases(extra) +
		`  *merge_requests\?state=opened*) cat '` + abs("mrs-opened.json") + `' ;;` + "\n" +
		`  *merge_requests/3966) cat '` + abs("mr-3966.json") + `' ;;` + "\n" +
		`  *merge_requests/3963) cat '` + abs("mr-3963.json") + `' ;;` + "\n" +
		`  *merge_requests\?state=merged*) cat '` + abs("mrs-merged.json") + `' ;;` + "\n" +
		`  *merge_requests\?state=closed*) cat '` + abs("mrs-closed.json") + `' ;;` + "\n" +
		`  *merge_requests/*/pipelines*) cat '` + abs("mr-pipelines.json") + `' ;;` + "\n" +
		`  *merge_requests/3967/notes*) cat '` + abs("mr-notes.json") + `' ;;` + "\n" +
		`  *merge_requests/3967) cat '` + abs("mr.json") + `' ;;` + "\n" +
		`  *pipelines/*/jobs*) cat '` + abs("jobs.json") + `' ;;` + "\n" +
		`  *issues\?state=opened*) cat '` + abs("issues.json") + `' ;;` + "\n" +
		`  *issues/8566/notes*) cat '` + abs("issue-notes.json") + `' ;;` + "\n" +
		`  *issues/8566) cat '` + abs("issue.json") + `' ;;` + "\n" +
		`  *) echo 'glab: 404 Not Found (HTTP 404)' >&2; exit 1 ;;` + "\n" +
		"esac\n"
	bin = filepath.Join(dir, "glab")
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return bin, calls
}

// gitLabRouter is a Router whose projects all have origin url, with glab as
// the fake above.
func gitLabRouter(t *testing.T, url string, hosts forge.Hosts) (*forge.Router, string) {
	bin, calls := fakeGlab(t)
	return forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: url}).url, Hosts: hosts},
		Bins:    map[forge.Kind]string{forge.KindGitLab: bin},
	}), calls
}

func glabCalls(t *testing.T, calls string) []string {
	t.Helper()
	b, err := os.ReadFile(calls)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func prNumbered(prs []forge.PR, n int) (forge.PR, bool) {
	for _, pr := range prs {
		if pr.Number == n {
			return pr, true
		}
	}
	return forge.PR{}, false
}

// A GitLab project's merge requests reach the card as pull requests do: open
// ones with branch, head, draft, fork, review and a CI verdict from their own
// latest pipeline; finished ones merged or closed (#454).
func TestGitLab_ListsMergeRequestsWithTheirCI_issue454(t *testing.T) {
	prs := gitLabPRs(t)

	mr, _ := prNumbered(prs, 3967)
	want := forge.PR{Number: 3967, Title: "chore: spec validation added", Branch: "7699-follow-up-validate-spec-for-components",
		State: forge.Open, CI: forge.CIPassing, Head: "cd159740322d05525356761f62e4f2ceac6228d6", Updated: mr.Updated}
	if !reflect.DeepEqual(mr, want) || mr.Updated.IsZero() {
		t.Errorf("!3967 = %+v\nwant %+v", mr, want)
	}
}

// gitLabPRs is the recorded project's merge requests through the fake glab.
func gitLabPRs(t *testing.T) []forge.PR {
	t.Helper()
	r, _ := gitLabRouter(t, "git@gitlab.com:gitlab-org/cli.git", nil)
	prs, err := r.ListPRs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return prs
}

// Draft, fork and a review asking for changes come through as marks.
func TestGitLab_MarksDraftForkAndReview_issue454(t *testing.T) {
	prs := gitLabPRs(t)

	if draft, _ := prNumbered(prs, 3966); !draft.Draft {
		t.Errorf("!3966 = %+v, want a draft", draft)
	}
	if fork, _ := prNumbered(prs, 3963); !fork.Fork || fork.Review != forge.ReviewChanges {
		t.Errorf("!3963 = %+v, want a fork with changes requested", fork)
	}
}

// The recently finished come too, merged with their time, for the card that
// says "merged" and for --stats' lead time (#332).
func TestGitLab_ListsTheRecentlyFinished_issue454(t *testing.T) {
	prs := gitLabPRs(t)

	finished := 0
	for _, pr := range prs {
		if pr.State == forge.Merged && !pr.MergedAt.IsZero() || pr.State == forge.Closed {
			finished++
		}
	}
	if finished != 4 {
		t.Errorf("read %d finished merge requests, want the 2 merged and 2 closed", finished)
	}
}

// A pipeline status maps to the card's CI mark, and one omatty does not know
// is running - unknown is never shown as passing.
func TestGitLab_PipelineStatusIsTheCIMark_issue454(t *testing.T) {
	for status, want := range map[string]forge.CIState{
		"success": forge.CIPassing, "skipped": forge.CIPassing,
		"failed": forge.CIFailing, "canceled": forge.CIFailing,
		"running": forge.CIRunning, "pending": forge.CIRunning, "created": forge.CIRunning,
		"manual": forge.CIRunning, "scheduled": forge.CIRunning, "something-new": forge.CIRunning,
	} {
		if got := forge.GitLabCI(status); got != want {
			t.Errorf("pipeline %q = %v, want %v", status, got, want)
		}
	}
}

// A pipeline that has finished is not asked again on the next poll: its
// verdict cannot change until the head does.
func TestGitLab_AFinishedPipelineIsNotAskedAgain_issue454(t *testing.T) {
	r, calls := gitLabRouter(t, "git@gitlab.com:gitlab-org/cli.git", nil)
	root := t.TempDir()
	if _, err := r.ListPRs(root); err != nil {
		t.Fatal(err)
	}
	before := len(glabCalls(t, calls))

	if _, err := r.ListPRs(root); err != nil {
		t.Fatal(err)
	}

	for _, call := range glabCalls(t, calls)[before:] {
		if strings.HasSuffix(call, "merge_requests/3967") {
			t.Errorf("a passing verdict was asked again: %s", call)
		}
	}
}

// The tracker's issues: number, title, labels, assignee, author.
func TestGitLab_ListsIssues_issue454(t *testing.T) {
	r, _ := gitLabRouter(t, "https://gitlab.com/gitlab-org/cli.git", nil)

	issues, err := r.ListIssues(t.TempDir())

	if err != nil || len(issues) != 3 || issues[0].Number != 8566 {
		t.Fatalf("issues = %+v, %v; want the three recorded, #8566 first", issues, err)
	}
	if got := issues[0].Labels; len(got) < 3 || got[0] != "automation:ml" {
		t.Errorf("labels = %v, want GitLab's own", got)
	}
	if issues[0].Author == "" || issues[0].URL == "" {
		t.Errorf("issue = %+v, want its author and its page", issues[0])
	}
}

// An item in full: its description as the body, the discussion without
// GitLab's system notes, and a merge request's jobs as its checks.
func TestGitLab_ReadsAMergeRequestInFull_issue454(t *testing.T) {
	r, _ := gitLabRouter(t, "git@gitlab.com:gitlab-org/cli.git", nil)

	d, err := r.ViewPR(t.TempDir(), 3967)

	if err != nil {
		t.Fatal(err)
	}
	if d.Number != 3967 || !strings.Contains(d.Body, "Describe your changes") || d.URL == "" {
		t.Errorf("detail = %+v, want !3967 with its description", d)
	}
	if len(d.Comments) != 2 || d.Comments[0].Author != "reviewer-one" {
		t.Errorf("comments = %+v, want the two that are not system notes", d.Comments)
	}
	if len(d.Checks) != 17 || d.Checks[2].Name != "tests:integration" || d.Checks[2].State != forge.CIPassing {
		t.Errorf("checks = %+v, want the pipeline's 17 jobs", d.Checks)
	}
}

// An issue in full, without its system note.
func TestGitLab_ReadsAnIssueInFull_issue454(t *testing.T) {
	r, _ := gitLabRouter(t, "git@gitlab.com:gitlab-org/cli.git", nil)

	d, err := r.ViewIssue(t.TempDir(), 8566)

	if err != nil || d.Number != 8566 || len(d.Comments) != 1 || d.Comments[0].Author != "someone-else" {
		t.Errorf("detail = %+v, %v; want #8566 with its one real comment", d, err)
	}
}

// A self-managed host is named to glab, and a nested group is one project path.
func TestGitLab_ASelfManagedNestedProjectIsAddressedWhole_issue454(t *testing.T) {
	r, calls := gitLabRouter(t, "git@git.corp.example:group/sub/project.git", forge.Hosts{"git.corp.example": forge.KindGitLab})

	_, _ = r.ListIssues(t.TempDir())

	got := glabCalls(t, calls)
	if len(got) == 0 || !strings.Contains(got[0], "--hostname git.corp.example") || !strings.Contains(got[0], "projects/group%2Fsub%2Fproject/issues") {
		t.Errorf("glab was called %q, want the host named and the path encoded whole", got)
	}
}

// GitLab's words: "merge request", written "!12".
func TestGitLab_SpeaksOfMergeRequests_issue454(t *testing.T) {
	r, _ := gitLabRouter(t, "git@gitlab.com:gitlab-org/cli.git", nil)
	root := t.TempDir()
	_, _ = r.ListIssues(root)

	if got := r.Label(root); got.Forge != "GitLab" || got.Short != "MR" || got.Ref(12) != "!12" {
		t.Errorf("Label = %+v, want GitLab's", got)
	}
}

// Without glab there is nothing to ask, said as a missing tool.
func TestGitLab_WithoutGlabIsAMissingTool_issue454(t *testing.T) {
	r := forge.NewTestRouter(forge.TestEnv{Options: forge.Options{Remote: (&FakeRemote{URL: "git@gitlab.com:g/p.git"}).url}})

	_, err := r.ListPRs(t.TempDir())

	var missing *forge.MissingToolError
	if !errors.As(err, &missing) || missing.Tool != "glab" {
		t.Errorf("error = %v, want glab missing", err)
	}
}

// glab's 404 is a project it cannot see - quiet, no forge to read - and its
// 401 is a login the forge refused.
func TestGitLab_GlabsHTTPStatusIsClassified_issue454(t *testing.T) {
	for status, check := range map[int]func(error) bool{
		404: func(err error) bool { return errors.Is(err, forge.ErrNoForge) },
		401: func(err error) bool { var a *forge.AuthError; return errors.As(err, &a) && a.Status == 401 },
	} {
		bin, _ := fakeGH(t, "", fmt.Sprintf("glab: %d Unauthorized (HTTP %d)", status, status), 1)
		r := forge.NewTestRouter(forge.TestEnv{
			Options: forge.Options{Remote: (&FakeRemote{URL: "git@gitlab.com:g/p.git"}).url},
			Bins:    map[forge.Kind]string{forge.KindGitLab: bin},
		})

		if _, err := r.ListIssues(t.TempDir()); !check(err) {
			t.Errorf("HTTP %d: error = %v", status, err)
		}
	}
}

// b opens the item's own page on the project's host.
func TestGitLab_BrowseOpensTheItemsPage_issue454(t *testing.T) {
	var opened []string
	bin, _ := fakeGlab(t)
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "git@gitlab.com:gitlab-org/cli.git"}).url},
		Bins:    map[forge.Kind]string{forge.KindGitLab: bin},
		Open:    func(url string) error { opened = append(opened, url); return nil },
	})

	_ = r.BrowsePR(t.TempDir(), 3967)
	_ = r.BrowseIssue(t.TempDir(), 8566)

	want := []string{"https://gitlab.com/gitlab-org/cli/-/merge_requests/3967", "https://gitlab.com/gitlab-org/cli/-/issues/8566"}
	if !reflect.DeepEqual(opened, want) {
		t.Errorf("opened %v, want %v", opened, want)
	}
}

// An item that is gone - deleted between the list poll and the read - is a
// failed read, not a project without a forge (#453's review).
func TestGitLab_AMissingItemIsNotAMissingForge_issue454(t *testing.T) {
	r, _ := gitLabRouter(t, "git@gitlab.com:gitlab-org/cli.git", nil)

	_, err := r.ViewIssue(t.TempDir(), 1)

	if err == nil || errors.Is(err, forge.ErrNoForge) || !errors.Is(err, forge.ErrNotFound) {
		t.Errorf("error = %v, want not found and not ErrNoForge", err)
	}
}

func extraCases(extra []glabRoute) string {
	var b strings.Builder
	for _, r := range extra {
		b.WriteString("  " + r.pattern + ") cat '" + r.file + "' ;;\n")
	}
	return b.String()
}

// Regression, #454's review: glab refuses a --hostname with a port ("invalid
// hostname"), so a self-managed GitLab on an https port failed every read.
// glab takes the API's port from its own per-host config; it is given the
// bare host.
func TestGitLab_AnInstanceOnAPortIsNamedToGlabBare_issue454(t *testing.T) {
	r, calls := gitLabRouter(t, "https://git.corp.example:8443/g/p.git", forge.Hosts{"git.corp.example": forge.KindGitLab})

	_, _ = r.ListIssues(t.TempDir())

	got := glabCalls(t, calls)
	if len(got) == 0 || !strings.Contains(got[0], "--hostname git.corp.example projects/") {
		t.Errorf("glab was called %q, want the bare host", got)
	}
}

// Regression, #454's review: the merge request's pipelines list can answer
// with the previous head's pipeline after a push, and that verdict was
// remembered under the new head. CI is the merge request's own
// head_pipeline, one per merge request, and none until the new one exists.
func TestGitLab_CIIsEachMergeRequestsHeadPipeline_issue454(t *testing.T) {
	prs := gitLabPRs(t)

	for n, want := range map[int]forge.CIState{3967: forge.CIPassing, 3966: forge.CIFailing, 3963: forge.CINone} {
		if pr, _ := prNumbered(prs, n); pr.CI != want {
			t.Errorf("!%d CI = %v, want %v", n, pr.CI, want)
		}
	}
}

// Regression, #454's review: a failed pipeline can be retried on the same
// head, so a failure is asked again; only a pass is kept.
func TestGitLab_AFailedPipelineIsAskedAgain_issue454(t *testing.T) {
	r, calls := gitLabRouter(t, "git@gitlab.com:gitlab-org/cli.git", nil)
	root := t.TempDir()
	_, _ = r.ListPRs(root)
	before := len(glabCalls(t, calls))

	_, _ = r.ListPRs(root)

	asked := strings.Join(glabCalls(t, calls)[before:], "\n")
	if !strings.Contains(asked, "merge_requests/3966\n") && !strings.HasSuffix(asked, "merge_requests/3966") {
		t.Errorf("!3966 (failed) was not asked again:\n%s", asked)
	}
}

// Regression, #454's review: altssh.gitlab.com is GitLab's ssh over 443, not
// an API host - glab dialled https://altssh.gitlab.com and failed, and browse
// opened a dead page. It is gitlab.com to everything but git.
func TestGitLab_AltSSHIsGitLabCom_issue454(t *testing.T) {
	var opened []string
	bin, calls := fakeGlab(t)
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "ssh://git@altssh.gitlab.com:443/gitlab-org/cli.git"}).url},
		Bins:    map[forge.Kind]string{forge.KindGitLab: bin},
		Open:    func(u string) error { opened = append(opened, u); return nil },
	})

	_, _ = r.ListIssues(t.TempDir())
	_ = r.BrowsePR(t.TempDir(), 3967)

	if got := glabCalls(t, calls); len(got) == 0 || !strings.Contains(got[0], "--hostname gitlab.com ") {
		t.Errorf("glab was called %q, want gitlab.com", got)
	}
	if len(opened) != 1 || opened[0] != "https://gitlab.com/gitlab-org/cli/-/merge_requests/3967" {
		t.Errorf("opened %v, want gitlab.com's page", opened)
	}
}

// Regression, #454's review: a hundred notes were read oldest first, so a long
// discussion kept its oldest hundred and dropped the newest, and said
// nothing. The newest hundred are kept, and a full page says there may be more.
func TestGitLab_ALongDiscussionKeepsTheNewestAndSaysItIsCut_issue454(t *testing.T) {
	notes := writeHundredNotes(t)
	bin, calls := fakeGlabWith(t, []glabRoute{{"*merge_requests/3967/notes*", notes}})
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "git@gitlab.com:gitlab-org/cli.git"}).url},
		Bins:    map[forge.Kind]string{forge.KindGitLab: bin},
	})

	d, err := r.ViewPR(t.TempDir(), 3967)

	if err != nil || !d.Truncated {
		t.Errorf("detail truncated = %v, %v; want a full page of notes marked as maybe more", d.Truncated, err)
	}
	if b, _ := os.ReadFile(calls); !strings.Contains(string(b), "notes?sort=desc") {
		t.Errorf("notes were asked oldest first:\n%s", b)
	}
}

func writeHundredNotes(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("[")
	for i := range 100 {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"id":%d,"body":"note %d","author":{"username":"u"},"created_at":"2026-09-26T01:00:00Z","system":false}`, i, i)
	}
	b.WriteString("]")
	p := filepath.Join(t.TempDir(), "notes-100.json")
	if err := os.WriteFile(p, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}
