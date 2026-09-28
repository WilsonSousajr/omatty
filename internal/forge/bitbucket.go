package forge

import (
	"context"
	"strconv"
)

// bbBackend is the Bitbucket Cloud backend (#460): pull requests and their
// commit statuses from https://api.bitbucket.org/2.0. Bitbucket has no
// official CLI, so it is REST only, and it keeps no issues any more
// (ErrNoTracker).
type bbBackend struct {
	f      fetcher
	remote Remote
	ci     *ciCache
	open   func(url string) error
}

// bitbucketLabel is Bitbucket's words: pull requests, "#12".
var bitbucketLabel = Label{Forge: "Bitbucket", Change: "pull request", Short: "PR", Sigil: "#"}

func (b bbBackend) repo() string { return "repositories/" + b.remote.Slug() }

// bbList is one page of Bitbucket's paged answer.
func bbList[T any](f fetcher, path string) func(context.Context, int) ([]T, error) {
	return func(ctx context.Context, page int) ([]T, error) {
		p, err := getJSON[bbPage[T]](ctx, f, path+"&pagelen="+strconv.Itoa(apiPage)+"&page="+strconv.Itoa(page))
		return p.Values, err
	}
}

func (b bbBackend) listPRs(ctx context.Context, _ string) ([]PR, error) {
	var finished bbPage[bbPR]
	var finishedErr error
	done := make(chan struct{})
	go func() { // alongside the open pages, as Gitea's are
		defer close(done)
		finished, finishedErr = getJSON[bbPage[bbPR]](ctx, b.f,
			b.repo()+"/pullrequests?state=MERGED&state=DECLINED&sort=-updated_on&pagelen="+finishedWindow)
	}()
	open, err := pages(ctx, 100, bbList[bbPR](b.f, b.repo()+"/pullrequests?state=OPEN"))
	<-done
	if err := firstErr(err, finishedErr); err != nil {
		return nil, repoMissing(err)
	}
	prs := append(foldBBPRs(open), foldBBPRs(finished.Values)...)
	b.ci.fill(ctx, prs, b.ciKey, b.statusCI)
	return prs, nil
}

func (b bbBackend) ciKey(pr PR) string {
	return b.remote.Host + "/" + b.remote.Slug() + "#" + strconv.Itoa(pr.Number) + "@" + pr.Head
}

// statusCI is the worst of the head commit's statuses, which Pipelines and
// any external CI both report to.
func (b bbBackend) statusCI(ctx context.Context, pr PR) (CIState, error) {
	statuses, err := b.statuses(ctx, pr.Head)
	if err != nil || len(statuses) == 0 {
		return CINone, err
	}
	worst := CINone
	for _, s := range statuses {
		worst = max(worst, bitbucketCI(s.State))
	}
	return worst, nil
}

func (b bbBackend) statuses(ctx context.Context, head string) ([]bbStatus, error) {
	p, err := getJSON[bbPage[bbStatus]](ctx, b.f, b.repo()+"/commit/"+head+"/statuses?pagelen="+strconv.Itoa(apiPage))
	return p.Values, err
}

// listIssues is ErrNoTracker: Bitbucket Cloud answers its issue API with 410
// Gone on every repository (CHANGE-3071, checked 2026-09-28).
func (b bbBackend) listIssues(context.Context, string) ([]Issue, error) { return nil, ErrNoTracker }

func (b bbBackend) viewIssue(context.Context, string, int) (Detail, error) {
	return Detail{}, ErrNoTracker
}

func (b bbBackend) viewPR(ctx context.Context, _ string, number int) (Detail, error) {
	path := b.repo() + "/pullrequests/" + strconv.Itoa(number)
	pr, err := getJSON[bbPR](ctx, b.f, path)
	if err != nil {
		return Detail{}, err
	}
	comments, _ := getJSON[bbPage[bbComment]](ctx, b.f, path+"/comments?pagelen=100")
	statuses, _ := b.statuses(ctx, pr.Source.Commit.Hash)
	return foldDetail(pr.flat(comments.Values, statuses)), nil
}

// browse opens the pull request's page; Bitbucket keeps no issues to open.
func (b bbBackend) browse(_ context.Context, _ string, number int, pr bool) error {
	if !pr {
		return ErrNoTracker
	}
	return b.open(webBase(b.remote) + "/" + b.remote.Slug() + "/pull-requests/" + strconv.Itoa(number))
}

// firstErr is the first of errs that is not nil.
func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// pickBitbucket is Bitbucket Cloud's REST API with BITBUCKET_TOKEN: an API
// token with its BITBUCKET_USER as Basic auth, or a repository or workspace
// access token alone as a bearer. There is no CLI to try first.
func (r *Router) pickBitbucket(remote Remote) (backend, error) {
	tok, _ := r.borrow([]string{"BITBUCKET_TOKEN"})
	if tok == "" {
		return nil, &MissingToolError{TokenEnv: "BITBUCKET_TOKEN"}
	}
	a := bearer(tok)
	if user, _ := r.borrow([]string{"BITBUCKET_USER"}); user != "" {
		a = basicAuth(user, tok)
	}
	f := restAPI{rest: r.rest, base: "https://api.bitbucket.org/2.0", auth: a, env: "BITBUCKET_TOKEN"}
	return bbBackend{f: f, remote: remote, ci: r.ci, open: r.open}, nil
}
