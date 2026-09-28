package forge

import (
	"context"
	"strconv"
)

// gtBackend is the Gitea backend, which is Forgejo's and Codeberg's too: they
// share an API lineage (#458). It reads Gitea's REST API (/api/v1) through
// the operator's tea or, without one, over HTTP (#459): the same paths either
// way, so one fold.
type gtBackend struct {
	f      fetcher
	remote Remote
	ci     *ciCache
	open   func(url string) error
}

// giteaPage is the most one page answers: Gitea's own default cap.
const giteaPage = 50

// codebergLabel and giteaLabel are the words: pull requests, "#12", and the
// forge named as the operator knows it. Forgejo is Gitea's kind (#451), so a
// self-hosted Forgejo is called Gitea; Codeberg, by host, is called Codeberg.
var (
	codebergLabel = Label{Forge: "Codeberg", Change: "pull request", Short: "PR", Sigil: "#"}
	giteaLabel    = Label{Forge: "Gitea", Change: "pull request", Short: "PR", Sigil: "#"}
)

func (g gtBackend) repo() string { return "repos/" + g.remote.Slug() }

func (g gtBackend) listPRs(ctx context.Context, _ string) ([]PR, error) {
	open, err := pages[gtPR](ctx, g.f, g.repo()+"/pulls?state=open", 100)
	if err != nil {
		return nil, repoMissing(err)
	}
	finished, err := getJSON[[]gtPR](ctx, g.f, g.repo()+"/pulls?state=closed&sort=recentupdate&limit="+finishedWindow)
	if err != nil {
		return nil, repoMissing(err)
	}
	prs := append(foldGTPRs(open), foldGTPRs(finished)...)
	g.ci.fill(ctx, prs, g.ciKey, g.statusCI)
	return prs, nil
}

func (g gtBackend) ciKey(pr PR) string {
	return g.remote.Host + "/" + g.remote.Slug() + "#" + strconv.Itoa(pr.Number) + "@" + pr.Head
}

// statusCI is the head commit's combined status, which Gitea and Forgejo
// Actions and any external CI all report to.
func (g gtBackend) statusCI(ctx context.Context, pr PR) (CIState, error) {
	st, err := getJSON[gtStatus](ctx, g.f, g.repo()+"/commits/"+pr.Head+"/status")
	if err != nil || st.TotalCount == 0 {
		return CINone, err
	}
	return giteaCI(st.State), nil
}

func (g gtBackend) listIssues(ctx context.Context, _ string) ([]Issue, error) {
	issues, err := pages[gtIssue](ctx, g.f, g.repo()+"/issues?state=open&type=issues", 100)
	if err != nil {
		return nil, repoMissing(err)
	}
	return foldGTIssues(issues), nil
}

func (g gtBackend) viewIssue(ctx context.Context, _ string, number int) (Detail, error) {
	item, err := getJSON[gtItem](ctx, g.f, g.repo()+"/issues/"+strconv.Itoa(number))
	if err != nil {
		return Detail{}, err
	}
	return foldDetail(item.flat(g.comments(ctx, number), nil)), nil
}

func (g gtBackend) viewPR(ctx context.Context, _ string, number int) (Detail, error) {
	item, err := getJSON[gtItem](ctx, g.f, g.repo()+"/pulls/"+strconv.Itoa(number))
	if err != nil {
		return Detail{}, err
	}
	st, _ := getJSON[gtStatus](ctx, g.f, g.repo()+"/commits/"+item.Head.SHA+"/status")
	return foldDetail(item.flat(g.comments(ctx, number), st.Statuses)), nil
}

// comments is an item's discussion; a pull request's lives on its issue.
// None when it cannot be read: the body is still worth reading.
func (g gtBackend) comments(ctx context.Context, number int) []gtComment {
	comments, _ := getJSON[[]gtComment](ctx, g.f, g.repo()+"/issues/"+strconv.Itoa(number)+"/comments")
	return comments
}

func (g gtBackend) browse(_ context.Context, _ string, number int, pr bool) error {
	kind := "issues"
	if pr {
		kind = "pulls"
	}
	return g.open(webBase(g.remote) + "/" + g.remote.Slug() + "/" + kind + "/" + strconv.Itoa(number))
}

// pages reads a list page by page until a short page or max: Gitea caps a
// page at fifty, and #358's windows ask for a hundred.
func pages[T any](ctx context.Context, f fetcher, path string, most int) ([]T, error) {
	var all []T
	for page := 1; len(all) < most; page++ {
		items, err := getJSON[[]T](ctx, f, path+"&limit="+strconv.Itoa(giteaPage)+"&page="+strconv.Itoa(page))
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) < giteaPage {
			break
		}
	}
	return all, nil
}
