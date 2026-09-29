package forge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// azBackend is the Azure DevOps backend (#456): pull requests with their build
// policies, and work items as the issues, from the organisation's REST API.
// It reads with the operator's own az login, the token az issues for Azure
// DevOps, or with a PAT from the environment (#457).
type azBackend struct {
	rest    restClient
	auth    auth
	env     string
	base    string // https://dev.azure.com/<org>, or a Server's collection
	project string
	repo    string
	host    string
	ci      *ciCache
	open    func(url string) error
}

// azureLabel is Azure's words: pull requests written "!12", since "#12" is a
// work item there.
var azureLabel = Label{Forge: "Azure DevOps", Change: "pull request", Short: "PR", Sigil: "!"}

// azureVersion pins every call's api-version, as Azure asks.
const azureVersion = "api-version=7.1"

// azureCoordinates is where an Azure remote puts its organisation, project
// and repository, in each of its shapes (#450): <org>/<project>/_git/<repo>
// on dev.azure.com (with _optimized or _full before the repository, when
// cloned that way), v3/<org>/<project>/<repo> over ssh, and
// [DefaultCollection/]<project>/_git/<repo> on org.visualstudio.com or a
// Server's collection.
func azureCoordinates(r Remote) (base, project, repo string, err error) {
	prefix, project, repo, ok := azurePath(r.Path)
	if !ok {
		return "", "", "", fmt.Errorf("forge: %q is not an Azure DevOps repository, want <org>/<project>/_git/<repo>: %w", r.Slug(), ErrNoForge)
	}
	switch {
	case r.Host == "dev.azure.com" || azureSSH(r.Host):
		if len(prefix) == 0 {
			return "", "", "", fmt.Errorf("forge: %q names no Azure DevOps organisation: %w", r.Slug(), ErrNoForge)
		}
		return "https://dev.azure.com/" + url.PathEscape(prefix[0]), project, repo, nil
	}
	return strings.TrimSuffix(webBase(r)+"/"+strings.Join(prefix, "/"), "/"), project, repo, nil
}

// azurePath splits a remote's path around its _git segment, or, for the ssh
// shape that has none, takes the last two segments as project and repository.
func azurePath(path []string) (prefix []string, project, repo string, ok bool) {
	for i, seg := range path {
		if seg == "_git" && i >= 1 && i < len(path)-1 {
			return path[:i-1], path[i-1], path[len(path)-1], true
		}
	}
	if len(path) >= 3 {
		return path[:len(path)-2], path[len(path)-2], path[len(path)-1], true
	}
	return nil, "", "", false
}

func (a azBackend) api(path string) string {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return a.base + "/" + url.PathEscape(a.project) + "/_apis/" + path + sep + azureVersion
}

func (a azBackend) repoAPI(path string) string {
	return a.api("git/repositories/" + url.PathEscape(a.repo) + "/" + path)
}

// azGet reads one Azure call and decodes it.
func azGet[T any](ctx context.Context, a azBackend, u string) (T, error) {
	var out T
	raw, err := a.rest.get(ctx, u, a.auth, a.env)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("forge: reading %s: %w", u, err)
	}
	return out, nil
}

func (a azBackend) listPRs(ctx context.Context, _ string) ([]PR, error) {
	var all []PR
	for _, q := range []string{"status=active&$top=100", "status=completed&$top=" + finishedWindow, "status=abandoned&$top=" + finishedWindow} {
		page, err := azGet[azList[azPR]](ctx, a, a.repoAPI("pullrequests?searchCriteria."+q))
		if err != nil {
			return nil, repoMissing(err)
		}
		all = append(all, foldAzPRs(page.Value)...)
	}
	a.ci.fill(ctx, all, a.ciKey, a.policyCI)
	return all, nil
}

func (a azBackend) ciKey(pr PR) string {
	return a.host + "/" + a.project + "/" + a.repo + "!" + strconv.Itoa(pr.Number) + "@" + pr.Head
}

// policyCI is the worst of the pull request's build policies: Azure's CI is a
// build validation policy on the target branch, evaluated per pull request.
func (a azBackend) policyCI(ctx context.Context, pr PR) (CIState, error) {
	builds, err := a.builds(ctx, pr.Number)
	if err != nil || len(builds) == 0 {
		return CINone, err
	}
	worst := CINone
	for _, b := range builds {
		worst = max(worst, azureCI(b.Status))
	}
	return worst, nil
}

// builds is the pull request's build policy evaluations. The artifact names
// the project by id, which is the repository's, read off the pull request.
func (a azBackend) builds(ctx context.Context, number int) ([]azEvaluation, error) {
	pr, err := azGet[azPR](ctx, a, a.repoAPI("pullrequests/"+strconv.Itoa(number)))
	if err != nil {
		return nil, err
	}
	artifact := "vstfs:///CodeReview/CodeReviewId/" + pr.Repository.Project.ID + "/" + strconv.Itoa(number)
	evals, err := azGet[azList[azEvaluation]](ctx, a, a.api("policy/evaluations?artifactId="+url.QueryEscape(artifact))+"-preview.1")
	return ciPolicies(evals.Value), err
}
