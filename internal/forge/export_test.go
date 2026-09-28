package forge

import (
	"context"
	"net/http/httptest"
	"strings"
	"time"
)

// NewRouterWithBin is NewRouter with bin in place of gh, so a test can stand a
// script in for it.
func NewRouterWithBin(o Options, bin string) *Router {
	r := NewRouter(o)
	r.bins[KindGitHub] = bin
	return r
}

// NewRouterWithSSH is NewRouterWithBin with ssh in place of the ssh binary an
// alias is resolved through (#576).
func NewRouterWithSSH(o Options, gh, ssh string) *Router {
	r := NewRouterWithBin(o, gh)
	r.sshBin = ssh
	return r
}

// NewCLIWithBin is a Router whose every project is on github.com and whose gh
// is bin: the gh backend's own tests, which predate the Router (#452), read the
// same with it in front.
func NewCLIWithBin(bin string) *Router { return NewCLIWithTimeout(bin, listTimeout) }

// NewCLIWithTimeout is NewCLIWithBin with its own bound, so a test of a
// stalled gh takes milliseconds rather than the thirty seconds a real one
// is given (#356).
func NewCLIWithTimeout(bin string, d time.Duration) *Router {
	onGitHub := func(string) (string, error) { return "https://github.com/WilsonSousajr/omatty.git", nil }
	r := NewRouterWithBin(Options{Remote: onGitHub}, bin)
	r.timeout = d
	return r
}

// FinishedFieldsInclude reports whether the finished-pull-request field set asks
// gh for name. The request is half of what makes a field reach the fold, and a
// fold test cannot see it (#332).
func FinishedFieldsInclude(name string) bool {
	for _, f := range strings.Split(finishedFields, ",") {
		if f == name {
			return true
		}
	}
	return false
}

// Auth and its builders are the REST transport's, named for the tests (#453).
type Auth = auth

var (
	PrivateToken = privateToken
	Bearer       = bearer
	TokenAuth    = tokenAuth
	BasicAuth    = basicAuth
	Anonymous    = anonymous
)

// BodyMax is the cap on one REST answer.
const BodyMax = bodyMax

// RESTGet is one GET through the REST transport inside bound, as a backend
// makes it, trusting srv's test certificate.
func RESTGet(srv *httptest.Server, url string, a Auth, tokenEnv string, bound time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), bound)
	defer cancel()
	c := newREST()
	c.http.Transport = srv.Client().Transport
	return c.get(ctx, url, a, tokenEnv)
}

// ErrNotFound is a 404, for the backend to judge.
var ErrNotFound = errNotFound
