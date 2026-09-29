package forge

import (
	"context"
	"net/url"
	"strconv"
)

// The bodies GitLab's merge request endpoints read. Every "no" is written
// out rather than left to a default, so a default that changes cannot turn one
// on: the source branch is kept, and a merge is now or not at all (#331).
type (
	glOpen struct {
		Source       string `json:"source_branch"`
		Target       string `json:"target_branch"`
		Title        string `json:"title"`
		RemoveSource bool   `json:"remove_source_branch"`
	}
	glMerge struct {
		RemoveSource         bool `json:"should_remove_source_branch"`
		WhenPipelineSucceeds bool `json:"merge_when_pipeline_succeeds"`
		AutoMerge            bool `json:"auto_merge"` // the newer name for the line above
	}
)

// createPR opens a merge request for head against base (#464). Its description
// is left empty: omatty composes nothing on the operator's behalf.
func (g glBackend) createPR(ctx context.Context, _, head, base, title string) (int, error) {
	mr, err := sendJSON[struct {
		IID int `json:"iid"`
	}](ctx, g.f, "POST", g.project()+"/merge_requests", glOpen{Source: head, Target: base, Title: title})
	if err != nil {
		return 0, err
	}
	return opened(mr.IID, g.remote.Host, "merge request")
}

// mergePR merges now, with the project's own merge method, which GitLab
// applies itself.
func (g glBackend) mergePR(ctx context.Context, _ string, number int) error {
	_, err := sendJSON[struct{}](ctx, g.f, "PUT", g.project()+"/merge_requests/"+strconv.Itoa(number)+"/merge", glMerge{})
	return err
}

// branchProtected is the branch's own protected flag, which GitLab sets for a
// wildcard rule too, and which needs no maintainer scope to read.
func (g glBackend) branchProtected(ctx context.Context, _, branch string) (bool, error) {
	b, err := getJSON[struct {
		Protected *bool `json:"protected"`
	}](ctx, g.f, g.project()+"/repository/branches/"+url.PathEscape(branch))
	return protectedFlag(b.Protected, err, branch)
}
