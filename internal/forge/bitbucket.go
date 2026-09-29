package forge

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

// bbPages reads up to most items one page at a time, asking for the next
// only when the last said there is one. Bitbucket counts every request against
// an hourly limit, and asking two pages at once, as Gitea's are, doubled the
// cost of every short list (#460's review).
func bbPages[T any](ctx context.Context, f fetcher, path string, most int) ([]T, error) {
	var all []T
	for page := 1; len(all) < most; page++ {
		p, err := getJSON[bbPage[T]](ctx, f, path+"&pagelen="+strconv.Itoa(apiPage)+"&page="+strconv.Itoa(page))
		if err != nil {
			return nil, err
		}
		all = append(all, p.Values...)
		if p.Next == "" {
			break
		}
	}
	return all, nil
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
	open, err := bbPages[bbPR](ctx, b.f, b.repo()+"/pullrequests?state=OPEN", 100)
	<-done
	if err := firstErr(err, finishedErr); err != nil {
		return nil, bbListErr(err)
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
	comments, err := getJSON[bbPage[bbComment]](ctx, b.f, path+"/comments?pagelen=100")
	statuses, _ := b.statuses(ctx, pr.Source.Commit.Hash)
	d := foldDetail(pr.flat(comments.Values, statuses))
	// Comments that failed, or have another page, are not the whole
	// discussion (#397; #460's review).
	d.Truncated = d.Truncated || err != nil || comments.Next != ""
	return d, nil
}

// bbListErr sorts a list's failure. Every read carries a token, and
// Bitbucket's 404 says "make sure you are authenticated": a token scoped to
// another repository is the likelier meaning than no forge, so it is a note
// naming the token (#460's review).
func bbListErr(err error) error {
	if errors.Is(err, errNotFound) {
		return &AuthError{Host: "api.bitbucket.org", TokenEnv: "BITBUCKET_TOKEN", Status: http.StatusNotFound}
	}
	return err
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

// pickBitbucket is Bitbucket's REST API; there is no CLI to try first.
// bitbucket.org is Cloud, read with BITBUCKET_TOKEN - an API token with its
// BITBUCKET_USER, the Atlassian account's email, as Basic auth, or an access
// token alone as a bearer. Any
// other host the operator named as bitbucket is Data Center (#461), whose API
// and whose token are its own.
func (r *Router) pickBitbucket(remote Remote) (backend, error) {
	if remote.Host != "bitbucket.org" {
		return r.dataCenter(remote)
	}
	a := r.bitbucketAuth("BITBUCKET_TOKEN", "BITBUCKET_USER")
	if a == nil {
		return nil, &MissingToolError{TokenEnv: "BITBUCKET_TOKEN"}
	}
	f := restAPI{rest: r.rest, base: "https://api.bitbucket.org/2.0", auth: a, env: "BITBUCKET_TOKEN"}
	return bbBackend{f: f, remote: remote, ci: r.ci, open: r.open}, nil
}

// bitbucketAuth is tokenEnv's token as Basic auth with userEnv's user, or as
// a bearer alone; nil when the token is unset.
func (r *Router) bitbucketAuth(tokenEnv, userEnv string) auth {
	tok, _ := r.borrow([]string{tokenEnv})
	if tok == "" {
		return nil
	}
	if user, _ := r.borrow([]string{userEnv}); user != "" {
		return basicAuth(user, tok)
	}
	return bearer(tok)
}

// dataCenter is the Data Center backend for remote. Its token is
// BITBUCKET_DC_TOKEN, sent only to the instance BITBUCKET_DC_URL names: one
// BITBUCKET_TOKEN went to Cloud and to every Data Center host alike, and an
// Atlassian API token covers the whole account (#461's review). An http
// instance is refused before any token is asked about.
func (r *Router) dataCenter(remote Remote) (backend, error) {
	contextPath, key, slug, err := dcCoordinates(remote)
	if err != nil {
		return nil, err
	}
	web, bound := r.dcWebRoot(remote, contextPath)
	if u, err := url.Parse(web); err != nil || u.Scheme != "https" {
		return nil, &PlainHTTPError{Host: strings.TrimPrefix(web, "http://"), TokenEnv: "BITBUCKET_DC_TOKEN"}
	}
	a := r.bitbucketAuth("BITBUCKET_DC_TOKEN", "BITBUCKET_DC_USER")
	if a == nil || !bound {
		return nil, &MissingToolError{TokenEnv: "BITBUCKET_DC_TOKEN for " + web}
	}
	f := restAPI{rest: r.rest, base: web + "/rest", auth: a, env: "BITBUCKET_DC_TOKEN"}
	return bdcBackend{f: f, remote: remote, web: web, key: key, slug: slug, ci: r.ci, open: r.open}, nil
}

// dcWebRoot is the instance's web root, and whether BITBUCKET_DC_URL names
// the instance. An https clone carries its own - port and context path; an
// ssh clone carries neither, so the variable gives it, and without it the
// root is https on the host (#461's review).
func (r *Router) dcWebRoot(remote Remote, contextPath string) (string, bool) {
	instance, _ := r.borrow([]string{"BITBUCKET_DC_URL"})
	bound := instance != "" && namesInstance(instance, remote)
	if remote.Scheme == "ssh" && bound {
		return strings.TrimSuffix(instance, "/"), true
	}
	web := webBase(remote)
	if contextPath != "" {
		web += "/" + contextPath
	}
	return web, bound
}
