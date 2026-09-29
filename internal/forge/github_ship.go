package forge

import (
	"context"
	"fmt"
	"strconv"
)

// GitHub's REST endpoints for #331's actions, for a machine with a token and
// no gh (#464): gh's own three calls, made over HTTP.
type (
	ghOpen struct {
		Title string `json:"title"`
		Head  string `json:"head"`
		Base  string `json:"base"`
	}
	ghMerge struct {
		Method string `json:"merge_method"`
	}
	ghMethods struct {
		Merge  bool `json:"allow_merge_commit"`
		Squash bool `json:"allow_squash_merge"`
		Rebase bool `json:"allow_rebase_merge"`
	}
)

// restAPI is the repository's REST API: api.github.com, a tenant's api host,
// or an Enterprise Server's /api/v3.
func (g ghHTTP) restAPI() restAPI {
	base := g.web() + "/api/v3"
	switch {
	case onGitHubCom(g.remote.Host):
		base = "https://api.github.com"
	case tenancy(g.remote.Host):
		base = "https://api." + g.remote.Host
	}
	return restAPI{rest: g.rest, base: base, auth: g.auth, env: g.env}
}

func (g ghHTTP) repoPath() string { return "repos/" + g.remote.Slug() }

// createPR opens a pull request for head against base, with no body: omatty
// composes nothing on the operator's behalf.
func (g ghHTTP) createPR(ctx context.Context, _, head, base, title string) (int, error) {
	pr, err := sendJSON[struct {
		Number int `json:"number"`
	}](ctx, g.restAPI(), "POST", g.repoPath()+"/pulls", ghOpen{Title: title, Head: head, Base: base})
	if err != nil {
		return 0, err
	}
	return opened(pr.Number, g.remote.Host, "pull request")
}

// mergePR merges now with the repository's own method: the first it allows,
// in the order GitHub's merge button offers them.
func (g ghHTTP) mergePR(ctx context.Context, _ string, number int) error {
	allowed, err := getJSON[ghMethods](ctx, g.restAPI(), g.repoPath())
	if err != nil {
		return err
	}
	method, err := allowed.first(g.remote.Slug())
	if err != nil {
		return err
	}
	_, err = sendJSON[struct{}](ctx, g.restAPI(), "PUT", g.repoPath()+"/pulls/"+strconv.Itoa(number)+"/merge", ghMerge{Method: method})
	return err
}

func (m ghMethods) first(slug string) (string, error) {
	switch {
	case m.Merge:
		return "merge", nil
	case m.Squash:
		return "squash", nil
	case m.Rebase:
		return "rebase", nil
	}
	return "", fmt.Errorf("forge: %s allows no merge method this token can see", slug)
}

// branchProtected is the branch's own protected flag, as gh reads it.
func (g ghHTTP) branchProtected(ctx context.Context, _, branch string) (bool, error) {
	b, err := getJSON[struct {
		Protected *bool `json:"protected"`
	}](ctx, g.restAPI(), g.repoPath()+"/branches/"+branch)
	return protectedFlag(b.Protected, err, branch)
}
