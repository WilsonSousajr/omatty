package forge

import (
	"context"
	"errors"
	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
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
	fin    *finishedCache
	open   func(url string) error
	// needsToken is what an anonymous REST read's refusal says is missing
	// (#459): nil when a token or tea reads.
	needsToken *dforge.MissingToolError
}

// codebergLabel and giteaLabel are the words: pull requests, "#12", and the
// forge named as the operator knows it. Forgejo is Gitea's kind (#451), so a
// self-hosted Forgejo is called Gitea; Codeberg, by host, is called Codeberg.
var (
	codebergLabel = dforge.Label{Forge: "Codeberg", Change: "pull request", Short: "PR", Sigil: "#"}
	giteaLabel    = dforge.Label{Forge: "Gitea", Change: "pull request", Short: "PR", Sigil: "#"}
)

func (g gtBackend) repo() string { return "repos/" + g.remote.Slug() }

// listErr sorts a list's failure. Gitea answers a list whose unit is off - a
// mirror's issues, an outside tracker - with 404, so a 404 on a repository
// that is there is an empty list, nil (#458's review), and one that cannot be
// judged is an outage, asked again next poll (#589's review). Read
// anonymously, a refusal or a 404 on a repository omatty cannot see is one
// that needs a token - Gitea answers a stranger's question about a private
// repository with 404 - so it says to set one rather than going quiet as a
// project on no forge (#459). With a token, a 404 is no forge.
func (g gtBackend) listErr(ctx context.Context, err error) error {
	if errors.Is(err, errNotFound) {
		if off, outage := g.unitOff(ctx); off || outage != nil {
			return outage
		}
	}
	if g.needsToken != nil && refusedAnonymously(err) {
		return g.needsToken
	}
	return repoMissing(err)
}

// unitOff reads the repository behind a list's 404: there, so the list's
// unit is off; gone or hidden, so the 404 stands; or unreadable for a reason
// that passes, returned as the outage it is.
func (g gtBackend) unitOff(ctx context.Context) (bool, error) {
	_, err := g.f.get(ctx, g.repo())
	var refused *dforge.AuthError
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, errNotFound), errors.As(err, &refused):
		return false, nil
	}
	return false, err
}

// refusedAnonymously is a 404, a 401 or a 403 - how Gitea turns a stranger
// away. A page where JSON was expected is not one: no token was refused.
func refusedAnonymously(err error) bool {
	var refused *dforge.AuthError
	return errors.Is(err, errNotFound) || errors.As(err, &refused) && (refused.Status == 401 || refused.Status == 403)
}

// listPRs reads the open pages, then the finished list only when the open
// set has moved (#594) - no longer alongside, since it is usually skipped.
func (g gtBackend) listPRs(ctx context.Context, _ string) ([]dforge.PR, error) {
	page, err := pages(ctx, 100, g.page(g.repo()+"/pulls?state=open"))
	if err != nil {
		return nil, g.listErr(ctx, err)
	}
	open := foldGTPRs(page)
	finished, err := g.fin.read(ctx, g.remote.Host+"/"+g.remote.Slug(), open, g.finished)
	if err != nil {
		return nil, g.listErr(ctx, err)
	}
	prs := append(open, finished...)
	if err := g.ci.fill(ctx, prs, g.ciKey, g.statusCI); err != nil {
		return nil, err
	}
	return prs, nil
}

// finished is the most recently closed pull requests, merged among them.
func (g gtBackend) finished(ctx context.Context) ([]dforge.PR, error) {
	closed, err := getJSON[[]gtPR](ctx, g.f, g.repo()+"/pulls?state=closed&sort=recentupdate&limit="+finishedWindow)
	return foldGTPRs(closed), err
}

func (g gtBackend) ciKey(pr dforge.PR) string {
	return g.remote.Host + "/" + g.remote.Slug() + "#" + strconv.Itoa(pr.Number) + "@" + pr.Head
}

// statusCI is the head commit's combined status, which Gitea and Forgejo
// Actions and any external CI all report to.
func (g gtBackend) statusCI(ctx context.Context, pr dforge.PR) (dforge.CIState, error) {
	st, err := getJSON[gtStatus](ctx, g.f, g.statusPath(pr.Head))
	if err != nil || st.TotalCount == 0 {
		return dforge.CINone, err
	}
	return giteaCI(st.State), nil
}

func (g gtBackend) listIssues(ctx context.Context, _ string) ([]dforge.Issue, error) {
	issues, err := pages(ctx, 100, gtPage[gtIssue](g.f, g.repo()+"/issues?state=open&type=issues"))
	if err == nil {
		return foldGTIssues(issues), nil
	}
	if err = g.listErr(ctx, err); err == nil {
		// The issues unit is off: a mirror's, or an outside tracker's, so the
		// issues are elsewhere, as Bitbucket's are (#458, #460).
		return nil, dforge.ErrNoTracker
	}
	return nil, err
}

func (g gtBackend) viewIssue(ctx context.Context, _ string, number int) (dforge.Detail, error) {
	item, err := getJSON[gtItem](ctx, g.f, g.repo()+"/issues/"+strconv.Itoa(number))
	if err != nil {
		return dforge.Detail{}, err
	}
	return g.detail(ctx, item, number, nil), nil
}

func (g gtBackend) viewPR(ctx context.Context, _ string, number int) (dforge.Detail, error) {
	item, err := getJSON[gtItem](ctx, g.f, g.repo()+"/pulls/"+strconv.Itoa(number))
	if err != nil {
		return dforge.Detail{}, err
	}
	st, _ := getJSON[gtStatus](ctx, g.f, g.statusPath(item.Head.SHA))
	return g.detail(ctx, item, number, st.Statuses), nil
}

// statusPath is a commit's combined status, a whole page of checks at once:
// Forgejo combines only the page it returns, thirty by default (#458).
func (g gtBackend) statusPath(sha string) string {
	return g.repo() + "/commits/" + sha + "/status?limit=" + strconv.Itoa(apiPage)
}

// detail folds an item with its comments; comments that cannot be read leave
// it marked as not whole, never as an item nobody discussed (#397).
func (g gtBackend) detail(ctx context.Context, item gtItem, number int, checks []gtCommitCheck) dforge.Detail {
	comments, err := getJSON[[]gtComment](ctx, g.f, g.repo()+"/issues/"+strconv.Itoa(number)+"/comments")
	d := foldDetail(item.flat(comments, checks))
	d.Truncated = d.Truncated || err != nil
	return d
}

func (g gtBackend) browse(_ context.Context, _ string, number int, pr bool) error {
	kind := "issues"
	if pr {
		kind = "pulls"
	}
	return g.open(webBase(g.remote) + "/" + g.remote.Slug() + "/" + kind + "/" + strconv.Itoa(number))
}

func (g gtBackend) page(path string) func(context.Context, int) ([]gtPR, error) {
	return gtPage[gtPR](g.f, path)
}

// gtPage reads one fifty-item page of a Gitea list.
func gtPage[T any](f fetcher, path string) func(context.Context, int) ([]T, error) {
	return func(ctx context.Context, page int) ([]T, error) {
		return getJSON[[]T](ctx, f, path+"&limit="+strconv.Itoa(apiPage)+"&page="+strconv.Itoa(page))
	}
}
