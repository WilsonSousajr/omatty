package forge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"
)

// NewRouterWithBin is NewRouter with bin in place of gh, so a test can stand a
// script in for it.
func NewRouterWithBin(o Options, bin string) *Router {
	r := NewTestRouter(TestEnv{Options: o})
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

// GitLabCI is a pipeline status as the card's CI mark.
var GitLabCI = gitlabCI

// GiteaCI is a commit status as the card's CI mark.
var GiteaCI = giteaCI

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

// TestEnv is everything a test stands in for around a Router (#462).
type TestEnv struct {
	Options
	// GH is the gh binary; empty is no gh on PATH.
	GH string
	// Bins is every other forge's CLI by kind; a kind absent is not on PATH.
	Bins map[Kind]string
	// Env is the environment tokens are borrowed from.
	Env map[string]string
	// API is a test server every HTTP request is sent to, whatever its host;
	// the host it was meant for arrives as X-Original-Host.
	API string
	// Open stands in for the browser.
	Open func(url string) error
}

// NewTestRouter is a Router over e.
func NewTestRouter(e TestEnv) *Router {
	r := NewRouter(e.Options)
	// Every CLI absent unless the test names it: this machine may have the
	// real ones, and a test never runs them. ssh too, for an alias lookup.
	for kind, name := range r.bins {
		r.bins[kind] = "/nonexistent/" + name
	}
	r.sshBin = "/nonexistent/ssh"
	if e.GH != "" {
		r.bins[KindGitHub] = e.GH
	}
	for kind, bin := range e.Bins {
		r.bins[kind] = bin
	}
	r.getenv = func(k string) string { return e.Env[k] }
	// No test reaches the network: without a test server, every request is
	// refused on a local port nothing listens on.
	if e.API == "" {
		e.API = "http://127.0.0.1:1"
	}
	r.rest = restTo(e.API)
	if e.Open != nil {
		r.open = e.Open
	}
	return r
}

// restTo is the REST client with every request sent to api, so production
// keeps no base-URL override and a test still sees the host it was meant for.
func restTo(api string) restClient {
	base, err := url.Parse(api)
	if err != nil {
		panic(err)
	}
	c := newREST()
	c.http.Transport = toServer{base: base}
	return c
}

type toServer struct{ base *url.URL }

func (t toServer) RoundTrip(req *http.Request) (*http.Response, error) {
	out := req.Clone(req.Context())
	out.Header.Set("X-Original-Host", req.URL.Host)
	out.Header.Set("X-Original-Scheme", req.URL.Scheme)
	out.URL.Scheme, out.URL.Host, out.Host = t.base.Scheme, t.base.Host, t.base.Host
	return http.DefaultTransport.RoundTrip(out)
}
