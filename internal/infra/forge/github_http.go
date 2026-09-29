package forge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ghHTTP is the GitHub backend over GitHub's own HTTP API, for a machine with
// a token in the environment and no gh (#462): CLI first, this second. It
// reads through GraphQL (see github_gql.go for why) and folds with gh's own
// folds, so a card cannot tell which transport filled it.
type ghHTTP struct {
	rest   restClient
	auth   auth
	env    string // the variable the token was borrowed from
	remote Remote
	open   func(url string) error
}

// gitHubTokens is the variables gh itself reads, in its order, so an operator
// who has used gh has probably set one: GH_TOKEN then GITHUB_TOKEN on
// github.com, GH_ENTERPRISE_TOKEN then GITHUB_ENTERPRISE_TOKEN on an
// Enterprise host.
func gitHubTokens(host string) []string {
	if onGitHubCom(host) || tenancy(host) {
		return []string{"GH_TOKEN", "GITHUB_TOKEN"}
	}
	return []string{"GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN"}
}

func onGitHubCom(host string) bool { return host == "github.com" || host == "ssh.github.com" }

// tenancy is GitHub Enterprise Cloud with data residency, <tenant>.ghe.com,
// which gh treats as github.com for its token and asks at api.<tenant>.ghe.com.
func tenancy(host string) bool { return strings.HasSuffix(host, ".ghe.com") }

// endpoint is the GraphQL URL: api.github.com's, a tenant's api host, or an
// Enterprise Server's own /api/graphql.
func (g ghHTTP) endpoint() string {
	switch {
	case onGitHubCom(g.remote.Host):
		return "https://api.github.com/graphql"
	case tenancy(g.remote.Host):
		return "https://api." + g.remote.Host + "/graphql"
	}
	return g.web() + "/api/graphql"
}

// web is the repository's host as a browser reaches it. An ssh remote's port
// is ssh's, so only an http(s) remote keeps its own.
func (g ghHTTP) web() string {
	if onGitHubCom(g.remote.Host) {
		return "https://github.com"
	}
	if g.remote.Scheme == "ssh" || g.remote.Port == "" {
		return "https://" + g.remote.Host
	}
	return g.remote.Scheme + "://" + g.remote.Host + ":" + g.remote.Port
}

func (g ghHTTP) listPRs(ctx context.Context, _ string) ([]PR, error) {
	repo, err := gql[gqlPRs](ctx, g, prsQuery, 0)
	if err != nil {
		return nil, err
	}
	open := make([]ghPR, 0, len(repo.Open.Nodes))
	for _, p := range repo.Open.Nodes {
		open = append(open, p.flat())
	}
	return append(foldPRs(open), foldPRs(repo.Finished.Nodes)...), nil
}

func (g ghHTTP) listIssues(ctx context.Context, _ string) ([]Issue, error) {
	repo, err := gql[gqlIssues](ctx, g, issuesQuery, 0)
	if err != nil {
		return nil, err
	}
	in := make([]ghIssue, 0, len(repo.Issues.Nodes))
	for _, is := range repo.Issues.Nodes {
		in = append(in, is.flat())
	}
	return foldIssues(in), nil
}

func (g ghHTTP) viewIssue(ctx context.Context, _ string, number int) (Detail, error) {
	return g.view(ctx, number)
}

func (g ghHTTP) viewPR(ctx context.Context, _ string, number int) (Detail, error) {
	return g.view(ctx, number)
}

func (g ghHTTP) view(ctx context.Context, number int) (Detail, error) {
	repo, err := gql[gqlItemRepo](ctx, g, itemQuery, number)
	if err != nil {
		return Detail{}, err
	}
	if repo.Item == nil {
		return Detail{}, fmt.Errorf("forge: %s has no item %d: %w", g.remote.Slug(), number, errNotFound)
	}
	d := foldDetail(repo.Item.flat())
	// One query reads a hundred comments; one with more is not the whole item,
	// and a short item must never read as the whole one (#397).
	d.Truncated = d.Truncated || repo.Item.Comments.TotalCount > len(repo.Item.Comments.Nodes)
	return d, nil
}

// browse opens the item's page. GitHub redirects between /issues/N and
// /pull/N, but the right one saves the operator a hop.
func (g ghHTTP) browse(_ context.Context, _ string, number int, pr bool) error {
	kind := "issues"
	if pr {
		kind = "pull"
	}
	return g.open(g.web() + "/" + g.remote.Slug() + "/" + kind + "/" + strconv.Itoa(number))
}

// gql asks one query about the remote's repository and decodes its answer.
func gql[T any](ctx context.Context, g ghHTTP, query string, number int) (T, error) {
	var zero T
	owner, name, err := g.ownerAndName()
	if err != nil {
		return zero, err
	}
	body, err := json.Marshal(gqlRequest{Query: query, Variables: gqlVars{Owner: owner, Name: name, Number: number}})
	if err != nil {
		return zero, fmt.Errorf("forge: encoding a GraphQL query: %w", err)
	}
	raw, err := g.rest.post(ctx, g.endpoint(), bytes.NewReader(body), g.auth, g.env)
	if err != nil {
		return zero, err
	}
	return decodeGQL[T](raw, g.remote.Slug())
}

// ownerAndName is a GitHub repository's two path segments.
func (g ghHTTP) ownerAndName() (string, string, error) {
	if len(g.remote.Path) != 2 {
		return "", "", fmt.Errorf("forge: %q is not owner/repo on %s, want two segments: %w", g.remote.Slug(), g.remote.Host, ErrNoForge)
	}
	return g.remote.Path[0], g.remote.Path[1], nil
}

// decodeGQL reads a GraphQL answer's repository, sorting its errors: a
// repository GitHub cannot resolve for this token is ErrNoForge, as a 404 is.
func decodeGQL[T any](raw []byte, slug string) (T, error) {
	var answer gqlAnswer[T]
	if err := json.Unmarshal(raw, &answer); err != nil {
		var zero T
		return zero, fmt.Errorf("forge: reading GitHub's answer about %s: %w", slug, err)
	}
	if err := answer.problem(slug); err != nil {
		var zero T
		return zero, err
	}
	return *answer.Data.Repository, nil
}

// problem is the answer's errors as one, or nil when it carries a repository.
// A NOT_FOUND without a repository is a repository this token cannot see -
// no forge. A NOT_FOUND beside one is an item that is gone, which its reader
// finds for itself: stopping the project for it was #462's review finding.
func (a gqlAnswer[T]) problem(slug string) error {
	if a.Data.Repository == nil && a.notFound() {
		return fmt.Errorf("forge: GitHub cannot see %s: %w", slug, ErrNoForge)
	}
	messages := make([]string, 0, len(a.Errors))
	for _, e := range a.Errors {
		if e.Type != "NOT_FOUND" {
			messages = append(messages, cleanLine(e.Message))
		}
	}
	if len(messages) > 0 || a.Data.Repository == nil {
		return fmt.Errorf("forge: GitHub answered about %s with no repository: %s", slug, strings.Join(messages, "; "))
	}
	return nil
}

func (a gqlAnswer[T]) notFound() bool {
	for _, e := range a.Errors {
		if e.Type == "NOT_FOUND" {
			return true
		}
	}
	return false
}

// The request and the answers, as GraphQL writes them. Each answer reshapes
// into gh's own JSON type, whose fold is already the one the gh path uses.
type (
	gqlRequest struct {
		Query     string  `json:"query"`
		Variables gqlVars `json:"variables"`
	}
	gqlVars struct {
		Owner  string `json:"owner"`
		Name   string `json:"name"`
		Number int    `json:"number,omitempty"`
	}
	gqlAnswer[T any] struct {
		Data   struct{ Repository *T } `json:"data"`
		Errors []gqlError              `json:"errors"`
	}
	gqlError struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	gqlNodes[T any] struct {
		Nodes []T `json:"nodes"`
	}
	gqlPRs struct {
		Open     gqlNodes[gqlPR] `json:"open"`
		Finished gqlNodes[ghPR]  `json:"finished"`
	}
	gqlPR struct {
		ghPR
		Commits gqlNodes[gqlCommit] `json:"commits"`
	}
	gqlCommit struct {
		Commit struct {
			StatusCheckRollup *struct {
				Contexts gqlNodes[check] `json:"contexts"`
			} `json:"statusCheckRollup"`
		} `json:"commit"`
	}
	gqlIssues struct {
		Issues gqlNodes[gqlIssue] `json:"issues"`
	}
	gqlIssue struct {
		Number    int               `json:"number"`
		Title     string            `json:"title"`
		URL       string            `json:"url"`
		UpdatedAt time.Time         `json:"updatedAt"`
		Author    ghUser            `json:"author"`
		Labels    gqlNodes[ghLabel] `json:"labels"`
		Assignees gqlNodes[ghUser]  `json:"assignees"`
	}
	gqlItemRepo struct {
		Item *gqlItem `json:"issueOrPullRequest"`
	}
	gqlItem struct {
		Number    int                 `json:"number"`
		Title     string              `json:"title"`
		Body      string              `json:"body"`
		URL       string              `json:"url"`
		CreatedAt time.Time           `json:"createdAt"`
		Author    ghUser              `json:"author"`
		Comments  gqlComments         `json:"comments"`
		Commits   gqlNodes[gqlCommit] `json:"commits"`
	}
	gqlComments struct {
		TotalCount int         `json:"totalCount"`
		Nodes      []ghComment `json:"nodes"`
	}
)

// flat is the pull request as gh reports it: its last commit's checks lifted
// to statusCheckRollup, which is where gh's own export puts them.
func (p gqlPR) flat() ghPR {
	out := p.ghPR
	out.StatusCheckRollup = checksOf(p.Commits)
	return out
}

func checksOf(commits gqlNodes[gqlCommit]) []check {
	if len(commits.Nodes) == 0 || commits.Nodes[0].Commit.StatusCheckRollup == nil {
		return nil
	}
	return commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes
}

func (is gqlIssue) flat() ghIssue {
	return ghIssue{
		Number: is.Number, Title: is.Title, URL: is.URL, UpdatedAt: is.UpdatedAt, Author: is.Author,
		Labels: is.Labels.Nodes, Assignees: is.Assignees.Nodes,
	}
}

func (it gqlItem) flat() ghDetail {
	return ghDetail{
		Number: it.Number, Title: it.Title, Body: it.Body, URL: it.URL, CreatedAt: it.CreatedAt,
		Author: it.Author, Comments: it.Comments.Nodes, Checks: checksOf(it.Commits),
	}
}
