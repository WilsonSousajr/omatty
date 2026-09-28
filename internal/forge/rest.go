package forge

import (
	"context"
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
// It follows no redirect, because Go carries Authorization across hosts only
// when it chooses to and PRIVATE-TOKEN always: a 3xx could otherwise hand the
// token to whatever host a response names.
type restClient struct{ http *http.Client }

func newREST() restClient {
	return restClient{http: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}}
}

// get is one bounded GET of url, sorted into a body or the error the UI acts
// on. tokenEnv names the variable a refusal is about.
func (c restClient) get(ctx context.Context, url string, a auth, tokenEnv string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("forge: building GET %s: %w", url, err)
	}
	req.Header.Set("Accept", "application/json")
	a(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("forge: GET %s gave no answer: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := answerError(resp, tokenEnv); err != nil {
		return nil, err
	}
	return readBounded(resp.Body, url)
}

// answerError sorts an answer that is not a body. 404 is a repository omatty
// cannot see - quiet, like a checkout on no forge. 401, a 403 that is not a
// rate limit, and a 2xx that is not JSON - Azure answers a bad PAT with its
// sign-in page - are the token refused, a note the operator can act on.
func answerError(resp *http.Response, tokenEnv string) error {
	host, status := resp.Request.URL.Host, resp.StatusCode
	switch {
	case status == http.StatusNotFound:
		return fmt.Errorf("forge: %s has no %s: %w", host, resp.Request.URL.Path, ErrNoForge)
	case status == http.StatusUnauthorized, status == http.StatusForbidden && !rateLimited(resp.Header):
		return &AuthError{Host: host, TokenEnv: tokenEnv, Status: status}
	case status >= http.StatusMultipleChoices:
		return fmt.Errorf("forge: %s answered %d %s", host, status, http.StatusText(status))
	case !strings.Contains(resp.Header.Get("Content-Type"), "json"):
		return &AuthError{Host: host, TokenEnv: tokenEnv, Status: status}
	}
	return nil
}

// rateLimited is a 403 that means "later", not "no": GitHub's primary limit
// empties X-RateLimit-Remaining, and its secondary limit sets Retry-After.
func rateLimited(h http.Header) bool {
	return h.Get("X-Ratelimit-Remaining") == "0" || h.Get("Retry-After") != ""
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
