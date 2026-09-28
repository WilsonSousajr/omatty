package forge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// bodyMax caps one REST answer. A page of a hundred pull requests from
// GitHub's REST API runs to a few megabytes, each carrying both repositories
// in full; eight is room for that and a bound on anything else, the argument
// the hook socket's 4 MiB cap makes (#55).
const bodyMax = 8 << 20

// auth puts a borrowed token where one forge reads it. It is built per call
// from the token read then, and dropped with the call: omatty stores no token
// (#453). A token rides in a header, never in a URL, where a proxy's log
// would keep it.
type auth func(*http.Request)

// privateToken is GitLab's header.
func privateToken(tok string) auth {
	return func(r *http.Request) { r.Header.Set("PRIVATE-TOKEN", tok) }
}

// bearer is GitHub's and Bitbucket Data Center's.
func bearer(tok string) auth {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+tok) }
}

// tokenAuth is Gitea's and Forgejo's.
func tokenAuth(tok string) auth {
	return func(r *http.Request) { r.Header.Set("Authorization", "token "+tok) }
}

// basicAuth is Azure's (an empty user and the PAT) and Bitbucket Cloud's.
func basicAuth(user, tok string) auth {
	return func(r *http.Request) { r.SetBasicAuth(user, tok) }
}

// anonymous is no header at all, for a public repository on Gitea (#459).
func anonymous() auth { return func(*http.Request) {} }

// restClient is the REST fallback's transport: one http.Client per Router,
// bounded by the call's context and never retried, the CLI path's #356 bound.
type restClient struct{ http *http.Client }

func newREST() restClient { return restClient{http: &http.Client{CheckRedirect: sameHostRedirect}} }

// sameHostRedirect follows a redirect only on the host it started on, over
// https, and at most three times. GitHub answers a renamed repository with a
// 301 to its new path, and git keeps working on the old origin, so a refused
// redirect would leave the list stale for good; but Go carries a custom
// header such as PRIVATE-TOKEN to any host, so one to another host is not
// followed - its 3xx comes back as an answer and is an error.
func sameHostRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 || req.URL.Host != via[0].URL.Host || req.URL.Scheme != "https" {
		return http.ErrUseLastResponse
	}
	return nil
}

// errNotFound is a 404. The transport cannot tell a repository it cannot see
// from an item that was deleted a moment ago, so the backend judges: a list's
// 404 is no forge, an item's only a missing item.
var errNotFound = errors.New("forge: not found")

// get is one bounded GET of url, sorted into a body or the error the UI acts
// on. tokenEnv names the variable a refusal is about.
func (c restClient) get(ctx context.Context, url string, a auth, tokenEnv string) ([]byte, error) {
	return c.send(ctx, http.MethodGet, url, nil, a, tokenEnv)
}

// post is get with a JSON body: GitHub's GraphQL reads are POSTs (#462).
func (c restClient) post(ctx context.Context, url string, body io.Reader, a auth, tokenEnv string) ([]byte, error) {
	return c.send(ctx, http.MethodPost, url, body, a, tokenEnv)
}

func (c restClient) send(ctx context.Context, method, url string, body io.Reader, a auth, tokenEnv string) ([]byte, error) {
	req, err := request(ctx, method, url, body, a, tokenEnv)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("forge: %s %s gave no answer: %w", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := answerError(resp, tokenEnv); err != nil {
		return nil, err
	}
	return readBounded(resp.Body, url)
}

// request builds one call with its auth applied, and refuses one that would
// carry a token over plain http, where the path or an HTTP_PROXY reads it.
func request(ctx context.Context, method, url string, body io.Reader, a auth, tokenEnv string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("forge: building %s %s: %w", method, url, err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	a(req)
	if req.URL.Scheme != "https" && carriesCredential(req.Header) {
		return nil, fmt.Errorf("forge: refusing to send %s's token to %s over plain http; the remote must be https", tokenEnv, req.URL.Host)
	}
	return req, nil
}

// carriesCredential is whether a request holds a token, in any scheme's header.
func carriesCredential(h http.Header) bool {
	return h.Get("Authorization") != "" || h.Get("PRIVATE-TOKEN") != ""
}

// answerError sorts an answer that is not a body. 401, and a 403 that is not a
// rate limit, are the token refused: a note the operator can act on. So is an
// HTML page in place of JSON, which is how Azure answers a bad PAT (#457).
// A 404 is errNotFound, for the backend to judge; anything else is an outage.
func answerError(resp *http.Response, tokenEnv string) error {
	host, status := resp.Request.URL.Host, resp.StatusCode
	switch {
	case status == http.StatusNotFound:
		return fmt.Errorf("forge: %s has no %s: %w", host, resp.Request.URL.Path, errNotFound)
	case status == http.StatusUnauthorized, status == http.StatusForbidden && !rateLimited(resp):
		return &AuthError{Host: host, TokenEnv: tokenEnv, Status: status}
	case status >= http.StatusMultipleChoices:
		return fmt.Errorf("forge: %s answered %d %s", host, status, http.StatusText(status))
	}
	return notJSON(resp, host, tokenEnv)
}

// notJSON sorts a 2xx that carries no JSON: Azure's 203 sign-in page, or any
// HTML, is a refused token; anything else - a 204, a proxy's blank page - is
// an answer omatty cannot read, not a verdict on the token.
func notJSON(resp *http.Response, host, tokenEnv string) error {
	contentType := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(contentType, "json"):
		return nil
	case resp.StatusCode == http.StatusNonAuthoritativeInfo, strings.HasPrefix(contentType, "text/html"):
		return &AuthError{Host: host, TokenEnv: tokenEnv, Status: resp.StatusCode}
	}
	return fmt.Errorf("forge: %s answered %d with no JSON (%q)", host, resp.StatusCode, contentType)
}

// rateLimited is a 403 that means "later", not "no": GitHub's primary limit
// empties X-RateLimit-Remaining, its secondary limit may set Retry-After, and
// may set neither and say so only in its body. That body is short, and read
// here within the same bound as any other.
func rateLimited(resp *http.Response) bool {
	if resp.Header.Get("X-Ratelimit-Remaining") == "0" || resp.Header.Get("Retry-After") != "" {
		return true
	}
	head, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	return strings.Contains(strings.ToLower(string(head)), "rate limit")
}

// readBounded reads at most bodyMax bytes, and refuses a longer answer rather
// than parsing its first part: half a JSON array either fails in the fold or
// folds into a list missing its tail.
func readBounded(body io.Reader, url string) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(body, bodyMax+1))
	if err != nil {
		return nil, fmt.Errorf("forge: reading %s: %w", url, err)
	}
	if len(b) > bodyMax {
		return nil, fmt.Errorf("forge: %s answered more than %d MiB", url, bodyMax>>20)
	}
	return b, nil
}
