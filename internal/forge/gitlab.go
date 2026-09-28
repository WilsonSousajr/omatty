package forge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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
var gitLabLabel = Label{Forge: "GitLab", Change: "merge request", Short: "MR", Sigil: "!"}

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

func (g glBackend) listPRs(ctx context.Context, _ string) ([]PR, error) {
	var all []PR
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
func (g glBackend) ciKey(pr PR) string {
	return g.remote.Host + "/" + g.remote.Slug() + "!" + strconv.Itoa(pr.Number) + "@" + pr.Head
}

// pipelineCI is the merge request's latest pipeline. Its own list, not the
// project's joined by SHA: a merge request pipeline runs on the merged
// result, whose SHA is not the head's (found reading gitlab-org/cli).
func (g glBackend) pipelineCI(ctx context.Context, pr PR) (CIState, error) {
	pipes, err := getJSON[[]glPipeline](ctx, g.f, g.mrPath(pr.Number)+"/pipelines?per_page=1")
	if err != nil || len(pipes) == 0 {
		return CINone, err
	}
	return gitlabCI(pipes[0].Status), nil
}

func (g glBackend) mrPath(number int) string {
	return g.project() + "/merge_requests/" + strconv.Itoa(number)
}

func (g glBackend) listIssues(ctx context.Context, _ string) ([]Issue, error) {
	issues, err := getJSON[[]glIssue](ctx, g.f, g.project()+"/issues?state=opened&per_page=100&order_by=created_at&sort=desc")
	if err != nil {
		return nil, repoMissing(err)
	}
	return foldGLIssues(issues), nil
}

func (g glBackend) viewIssue(ctx context.Context, _ string, number int) (Detail, error) {
	path := g.project() + "/issues/" + strconv.Itoa(number)
	item, err := getJSON[glItem](ctx, g.f, path)
	if err != nil {
		return Detail{}, err
	}
	return foldDetail(item.flat(g.notes(ctx, path), nil)), nil
}

func (g glBackend) viewPR(ctx context.Context, _ string, number int) (Detail, error) {
	item, err := getJSON[glItem](ctx, g.f, g.mrPath(number))
	if err != nil {
		return Detail{}, err
	}
	return foldDetail(item.flat(g.notes(ctx, g.mrPath(number)), g.jobs(ctx, number))), nil
}

// notes is an item's discussion without GitLab's system notes ("added 1
// commit"). A refused read leaves the item without comments rather than
// failing it: GitLab answers an anonymous notes request with 401 even on a
// public project, and the body is still worth reading.
func (g glBackend) notes(ctx context.Context, itemPath string) []glNote {
	notes, err := getJSON[[]glNote](ctx, g.f, itemPath+"/notes?sort=asc&per_page=100")
	var refused *AuthError
	if err != nil && errors.As(err, &refused) {
		return nil
	}
	return notes
}

// jobs is the merge request's latest pipeline's jobs, its checks. None when it
// has no pipeline, or the pipeline cannot be read.
func (g glBackend) jobs(ctx context.Context, number int) []glJob {
	pipes, err := getJSON[[]glPipeline](ctx, g.f, g.mrPath(number)+"/pipelines?per_page=1")
	if err != nil || len(pipes) == 0 {
		return nil
	}
	jobs, _ := getJSON[[]glJob](ctx, g.f, g.project()+"/pipelines/"+strconv.Itoa(pipes[0].ID)+"/jobs?per_page=100")
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
// is ssh's, so only an http(s) remote keeps its own.
func webBase(r Remote) string {
	if r.Scheme == "ssh" || r.Port == "" {
		return "https://" + r.Host
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
