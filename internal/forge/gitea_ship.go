package forge

import (
	"context"
	"strconv"
)

// The bodies Gitea's pull request endpoints read. Every "no" is written out
// (#331): the head branch is kept, and a merge is now or not at all.
type (
	gtOpen struct {
		Head  string `json:"head"`
		Base  string `json:"base"`
		Title string `json:"title"`
	}
	gtMerge struct {
		Do                string `json:"Do"`
		DeleteBranch      bool   `json:"delete_branch_after_merge"`
		WhenChecksSucceed bool   `json:"merge_when_checks_succeed"`
	}
)

// createPR opens a pull request for head against base (#464).
func (g gtBackend) createPR(ctx context.Context, _, head, base, title string) (int, error) {
	pr, err := sendJSON[struct {
		Number int `json:"number"`
	}](ctx, g.f, "POST", g.repo()+"/pulls", gtOpen{Head: head, Base: base, Title: title})
	if err != nil {
		return 0, err
	}
	return opened(pr.Number, g.remote.Host, "pull request")
}

// mergePR merges with the repository's own default style. Gitea's merge takes
// no default of its own - "Do" is required - so the repository is read for it.
func (g gtBackend) mergePR(ctx context.Context, _ string, number int) error {
	repo, err := getJSON[struct {
		Style string `json:"default_merge_style"`
	}](ctx, g.f, g.repo())
	if err != nil {
		return err
	}
	style := repo.Style
	if style == "" { // an instance older than the setting merges with a merge commit
		style = "merge"
	}
	_, err = sendJSON[struct{}](ctx, g.f, "POST", g.repo()+"/pulls/"+strconv.Itoa(number)+"/merge", gtMerge{Do: style})
	return err
}

// branchProtected is the branch's own protected flag.
func (g gtBackend) branchProtected(ctx context.Context, _, branch string) (bool, error) {
	b, err := getJSON[struct {
		Protected *bool `json:"protected"`
	}](ctx, g.f, g.repo()+"/branches/"+branch)
	return protectedFlag(b.Protected, err, branch)
}
