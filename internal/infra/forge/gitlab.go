package forge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
	"net/url"
	"slices"
	"strconv"
)

// glBackend is the GitLab backend (#454): merge requests, issues and items
// from GitLab's REST API (/api/v4), through the operator's glab or, when glab
// is absent, over HTTP (#455). Nested groups are one project path, and a
// self-managed host is named to glab.
type glBackend struct {
	f      fetcher
	remote Remote
	ci     *ciCache
	open   func(url string) error
}

// gitLabLabel is GitLab's words: a merge request, written "!12".
var gitLabLabel = dforge.Label{Forge: "GitLab", Change: "merge request", Short: "MR", Sigil: "!"}

// glLists is each merge request list ListPRs reads: every open one, newest
// first, then the recently finished of each kind - #358's windows.
var glLists = []string{
	"state=opened&per_page=100&order_by=created_at&sort=desc",
	"state=merged&per_page=" + finishedWindow + "&order_by=updated_at&sort=desc",
	"state=closed&per_page=" + finishedWindow + "&order_by=updated_at&sort=desc",
}

// project is the API's name for the repository: its whole path, encoded, so
// group/sub/project is one segment.
func (g glBackend) project() string { return "projects/" + url.PathEscape(g.remote.Slug()) }

func (g glBackend) listPRs(ctx context.Context, _ string) ([]dforge.PR, error) {
	var all []dforge.PR
	for _, list := range glLists {
		mrs, err := getJSON[[]glMR](ctx, g.f, g.project()+"/merge_requests?"+list)
		if err != nil {
			return nil, repoMissing(err)
		}
		all = append(all, foldMRs(mrs)...)
	}
	g.ci.fill(ctx, all, g.ciKey, g.pipelineCI)
	return all, nil
}

// ciKey names a merge request's head across every project the Router reads.
func (g glBackend) ciKey(pr dforge.PR) string {
	return g.remote.Host + "/" + g.remote.Slug() + "!" + strconv.Itoa(pr.Number) + "@" + pr.Head
}

// pipelineCI is the merge request's own head_pipeline. Not the project's
// pipelines joined by SHA - a merge request pipeline runs on the merged
// result, whose SHA is not the head's (found reading gitlab-org/cli) - and not
// the merge request's pipelines list, which after a push can answer with the
// previous head's (#454's review). None until the new head has one.
func (g glBackend) pipelineCI(ctx context.Context, pr dforge.PR) (dforge.CIState, error) {
	mr, err := getJSON[glItem](ctx, g.f, g.mrPath(pr.Number))
	if err != nil || mr.HeadPipeline == nil {
		return dforge.CINone, err
	}
	return gitlabCI(mr.HeadPipeline.Status), nil
}

func (g glBackend) mrPath(number int) string {
	return g.project() + "/merge_requests/" + strconv.Itoa(number)
}

func (g glBackend) listIssues(ctx context.Context, _ string) ([]dforge.Issue, error) {
	issues, err := getJSON[[]glIssue](ctx, g.f, g.project()+"/issues?state=opened&per_page=100&order_by=created_at&sort=desc")
	if err != nil {
		return nil, repoMissing(err)
	}
	return foldGLIssues(issues), nil
}

func (g glBackend) viewIssue(ctx context.Context, _ string, number int) (dforge.Detail, error) {
	path := g.project() + "/issues/" + strconv.Itoa(number)
	item, err := getJSON[glItem](ctx, g.f, path)
	if err != nil {
		return dforge.Detail{}, err
	}
	notes, cut := g.notes(ctx, path)
	d := foldDetail(item.flat(notes, nil))
	d.Truncated = d.Truncated || cut
	return d, nil
}

func (g glBackend) viewPR(ctx context.Context, _ string, number int) (dforge.Detail, error) {
	item, err := getJSON[glItem](ctx, g.f, g.mrPath(number))
	if err != nil {
		return dforge.Detail{}, err
	}
	notes, cut := g.notes(ctx, g.mrPath(number))
	d := foldDetail(item.flat(notes, g.jobs(ctx, item.HeadPipeline)))
	d.Truncated = d.Truncated || cut
	return d, nil
}

// notes is an item's newest hundred notes, oldest first, and whether that may
// not be all of them. Read newest first, because a long discussion's end is
// what a reader came for (#454's review). A full page may have more behind it,
// and a failed read is cut too - except a refusal: GitLab answers an anonymous
// notes request with 401 even on a public project, and there is nothing more
// to say than that the item has no comments to show.
func (g glBackend) notes(ctx context.Context, itemPath string) ([]glNote, bool) {
	notes, err := getJSON[[]glNote](ctx, g.f, itemPath+"/notes?sort=desc&per_page=100")
	var refused *dforge.AuthError
	if err != nil {
		return nil, !errors.As(err, &refused)
	}
	slices.Reverse(notes)
	return notes, len(notes) == 100
}

// jobs is the head pipeline's jobs, its checks: none without one, or when they
// cannot be read.
func (g glBackend) jobs(ctx context.Context, head *glPipeline) []glJob {
	if head == nil {
		return nil
	}
	jobs, _ := getJSON[[]glJob](ctx, g.f, g.project()+"/pipelines/"+strconv.Itoa(head.ID)+"/jobs?per_page=100")
	return jobs
}

// browse opens the item's own page on the project's host.
func (g glBackend) browse(_ context.Context, _ string, number int, pr bool) error {
	kind := "issues"
	if pr {
		kind = "merge_requests"
	}
	return g.open(webBase(g.remote) + "/" + g.remote.Slug() + "/-/" + kind + "/" + strconv.Itoa(number))
}

// webBase is the remote's host as a browser reaches it. An ssh remote's port
// is ssh's, so only an http(s) remote keeps its own. An http remote stays
// http: an http-only host does not answer https, and asking it there failed
// every poll (#584). No token is sent over it - request refuses that.
func webBase(r Remote) string {
	switch {
	case r.Scheme == "ssh":
		return "https://" + r.Host
	case r.Port == "":
		return r.Scheme + "://" + r.Host
	}
	return r.Scheme + "://" + r.Host + ":" + r.Port
}

// getJSON reads one path through f and decodes it.
func getJSON[T any](ctx context.Context, f fetcher, path string) (T, error) {
	var out T
	raw, err := f.get(ctx, path)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("forge: reading %s: %w", path, err)
	}
	return out, nil
}
