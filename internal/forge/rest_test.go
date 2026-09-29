package forge_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// secret is the token every REST test borrows: distinctive, so a leak of any
// part of it into an error or a log line is found by a plain substring search.
const secret = "tok-SECRET-4e1f"

// FakeAPI is a forge's REST API as a test server: it answers every request
// with status, contentType and body, and keeps the request it was sent.
type FakeAPI struct {
	Status      int
	ContentType string
	Body        string
	Header      http.Header
	Delay       time.Duration
	Got         *http.Request
}

func (f *FakeAPI) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.Got = r
		time.Sleep(f.Delay)
		for k, v := range f.Header {
			w.Header()[k] = v
		}
		w.Header().Set("Content-Type", f.ContentType)
		w.WriteHeader(f.Status)
		_, _ = w.Write([]byte(f.Body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fetch is one GET through the REST transport with GITLAB_TOKEN's auth.
func fetch(t *testing.T, f *FakeAPI, bound time.Duration) ([]byte, error) {
	t.Helper()
	srv := f.serve(t)
	return forge.RESTGet(srv, srv.URL+"/api/v4/projects/g%2Fp", forge.PrivateToken(secret), "GITLAB_TOKEN", bound)
}

// The token rides in the forge's own header and nowhere else: not in the URL,
// where a proxy log would keep it.
func TestREST_SendsTheTokenInItsHeader_issue453(t *testing.T) {
	f := &FakeAPI{Status: 200, ContentType: "application/json", Body: `{"ok":true}`}

	body, err := fetch(t, f, time.Second)

	if err != nil || string(body) != `{"ok":true}` {
		t.Fatalf("GET = %q, %v; want the body", body, err)
	}
	if got := f.Got.Header.Get("PRIVATE-TOKEN"); got != secret {
		t.Errorf("PRIVATE-TOKEN = %q, want the borrowed token", got)
	}
	if strings.Contains(f.Got.URL.String(), secret) {
		t.Errorf("the token is in the URL %s", f.Got.URL)
	}
}

// Each auth scheme a forge uses puts the token where that forge reads it.
func TestREST_EachAuthSchemeSetsItsHeader_issue453(t *testing.T) {
	for name, tt := range map[string]struct {
		auth forge.Auth
		want string
	}{
		"bearer": {forge.Bearer(secret), "Bearer " + secret},
		"token":  {forge.TokenAuth(secret), "token " + secret},
		// Built at run time: a literal base64 credential reads to a secret
		// scanner as a leaked one, and this is a fake.
		"basic": {forge.BasicAuth("", secret), "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+secret))},
	} {
		f := &FakeAPI{Status: 200, ContentType: "application/json", Body: "{}"}
		srv := f.serve(t)
		if _, err := forge.RESTGet(srv, srv.URL, tt.auth, "T", time.Second); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := f.Got.Header.Get("Authorization"); got != tt.want {
			t.Errorf("%s: Authorization = %q, want %q", name, got, tt.want)
		}
	}
}

// Anonymous sends no Authorization at all: Gitea's public repositories are read
// without a token (#459), and an empty header is still a header.
func TestREST_AnonymousSendsNoAuthorization_issue453(t *testing.T) {
	f := &FakeAPI{Status: 200, ContentType: "application/json", Body: "[]"}
	srv := f.serve(t)

	if _, err := forge.RESTGet(srv, srv.URL, forge.Anonymous(), "GITEA_TOKEN", time.Second); err != nil {
		t.Fatal(err)
	}
	if _, sent := f.Got.Header["Authorization"]; sent {
		t.Error("an anonymous request sent Authorization")
	}
}

// Each answer is sorted the way the UI needs it. 401 and 403 are the token
// refused - a visible note - except a 403 that is a rate limit, by header or by
// GitHub's own words, which is an outage to wait out. A 2xx that is HTML is
// Azure's sign-in page for a bad PAT (#457). 404 is left to the backend: a
// missing repository is no forge, a missing item is only a missing item.
func TestREST_ClassifiesTheAnswer_issue453(t *testing.T) {
	for _, tt := range []struct {
		status         int
		contentType    string
		header         http.Header
		body           string
		auth, notFound bool
	}{
		{404, "application/json", nil, "", false, true},
		{401, "application/json", nil, "", true, false},
		{403, "application/json", nil, `{"message":"Resource not accessible by token"}`, true, false},
		{203, "text/html; charset=utf-8", nil, "<html>sign in</html>", true, false},
		{403, "application/json", http.Header{"X-Ratelimit-Remaining": {"0"}}, "", false, false},
		{403, "application/json", http.Header{"Retry-After": {"60"}}, "", false, false},
		{403, "application/json", nil, `{"message":"You have exceeded a secondary rate limit."}`, false, false},
		{429, "application/json", nil, "", false, false},
		{204, "", nil, "", false, false},
		{502, "text/html", nil, "", false, false},
	} {
		f := &FakeAPI{Status: tt.status, ContentType: tt.contentType, Header: tt.header, Body: tt.body}

		_, err := fetch(t, f, time.Second)

		var auth *forge.AuthError
		if err == nil || errors.As(err, &auth) != tt.auth || errors.Is(err, forge.ErrNotFound) != tt.notFound || errors.Is(err, forge.ErrNoForge) {
			t.Errorf("%d %s %q: error = %v, want auth=%v notFound=%v", tt.status, tt.contentType, tt.body, err, tt.auth, tt.notFound)
		}
	}
}

// A token is never sent over plain http, where anyone on the path, or an
// HTTP_PROXY, reads the headers. Anonymous reads may go: they carry nothing.
func TestREST_NeverSendsATokenOverPlainHTTP_issue453(t *testing.T) {
	plain := &FakeAPI{Status: 200, ContentType: "application/json", Body: "[]"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		plain.Got = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(srv.Close)

	_, err := forge.RESTGet(srv, srv.URL, forge.PrivateToken(secret), "GITLAB_TOKEN", time.Second)

	if err == nil || !strings.Contains(err.Error(), "https") || plain.Got != nil {
		t.Errorf("error = %v, server got a request %v; want the token refused before sending", err, plain.Got != nil)
	}
	// Typed, so the UI stops the project with a note: no poll changes a
	// remote's scheme, and an untyped refusal was asked again forever (#584).
	var refused *forge.PlainHTTPError
	if !errors.As(err, &refused) || refused.TokenEnv != "GITLAB_TOKEN" || refused.Host == "" {
		t.Errorf("error = %#v, want a PlainHTTPError naming the host and GITLAB_TOKEN (#584)", err)
	}
	if _, err := forge.RESTGet(srv, srv.URL, forge.Anonymous(), "GITEA_TOKEN", time.Second); err != nil {
		t.Errorf("an anonymous read over http: %v, want it sent", err)
	}
}

// A redirect on the same host is followed with the token - GitHub answers a
// renamed repository with a 301 to its new path, and git keeps working on the
// old origin. One to another host is not.
func TestREST_FollowsARedirectOnlyOnItsOwnHost_issue453(t *testing.T) {
	var landed *http.Request
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old" {
			http.Redirect(w, r, "/new", http.StatusMovedPermanently)
			return
		}
		landed = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(srv.Close)

	if _, err := forge.RESTGet(srv, srv.URL+"/old", forge.PrivateToken(secret), "GITLAB_TOKEN", time.Second); err != nil {
		t.Fatalf("a same-host redirect: %v, want it followed", err)
	}
	if landed == nil || landed.Header.Get("PRIVATE-TOKEN") != secret {
		t.Error("the redirect was not followed with the token")
	}
}

// A redirect to another host is refused, and that host is never sent the token.
func TestREST_ARedirectToAnotherHostIsRefused_issue453(t *testing.T) {
	var elsewhere *http.Request
	other := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { elsewhere = r }))
	t.Cleanup(other.Close)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, strings.Replace(other.URL, "127.0.0.1", "localhost", 1)+"/stolen", http.StatusMovedPermanently)
	}))
	t.Cleanup(srv.Close)

	_, err := forge.RESTGet(srv, srv.URL, forge.PrivateToken(secret), "GITLAB_TOKEN", time.Second)

	if err == nil || !strings.Contains(err.Error(), "301") || elsewhere != nil {
		t.Errorf("error = %v, other host reached %v; want the redirect refused", err, elsewhere != nil)
	}
}

// A body of exactly the cap is read: the cap is a limit, not a limit minus one.
func TestREST_ABodyOfExactlyTheCapIsRead_issue453(t *testing.T) {
	f := &FakeAPI{Status: 200, ContentType: "application/json", Body: "[" + strings.Repeat(" ", forge.BodyMax-2) + "]"}

	if body, err := fetch(t, f, 5*time.Second); err != nil || len(body) != forge.BodyMax {
		t.Errorf("read %d bytes, %v; want all %d", len(body), err, forge.BodyMax)
	}
}

// A refused token names the host and the variable, so the operator knows which
// of their tokens to fix.
func TestAuthError_NamesTheHostAndTheVariable_issue453(t *testing.T) {
	err := &forge.AuthError{Host: "gitlab.com", TokenEnv: "GITLAB_TOKEN", Status: 401}

	if got := err.Error(); got != "forge: gitlab.com refused GITLAB_TOKEN (401)" {
		t.Errorf("Error() = %q", got)
	}
}

// A body over the cap is an error, never a truncated parse: half a JSON array
// either fails in the fold or, worse, folds into a list missing its tail.
func TestREST_AnOversizedBodyIsAnError_issue453(t *testing.T) {
	f := &FakeAPI{Status: 200, ContentType: "application/json", Body: "[" + strings.Repeat(" ", forge.BodyMax) + "]"}

	if _, err := fetch(t, f, 5*time.Second); err == nil || !strings.Contains(err.Error(), "more than") {
		t.Errorf("error = %v, want the cap named", err)
	}
}

// A forge that does not answer inside the bound is an error, the same #356
// bound the CLI path has, and there is no retry.
func TestREST_AStalledForgeGivesUp_issue453(t *testing.T) {
	f := &FakeAPI{Status: 200, ContentType: "application/json", Body: "[]", Delay: 500 * time.Millisecond}
	start := time.Now()

	_, err := fetch(t, f, 50*time.Millisecond)

	if err == nil || time.Since(start) > 400*time.Millisecond {
		t.Errorf("error = %v after %v, want a timeout well inside the delay", err, time.Since(start))
	}
}

// A redirect is not followed: Go carries Authorization across hosts only when it
// chooses to, and PRIVATE-TOKEN always, so following one could hand the token
// to whatever host a response names.
func TestREST_ARedirectIsNotFollowed_issue453(t *testing.T) {
	f := &FakeAPI{Status: 302, ContentType: "text/html", Header: http.Header{"Location": {"https://elsewhere.example/"}}}

	if _, err := fetch(t, f, time.Second); err == nil || !strings.Contains(err.Error(), "302") {
		t.Errorf("error = %v, want the redirect refused by its status", err)
	}
}

// The token never reaches an error or the log, whatever the forge answers.
func TestREST_TheTokenIsNeverInAnErrorOrTheLog_issue453(t *testing.T) {
	var log bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	for _, f := range []*FakeAPI{
		{Status: 401, ContentType: "application/json", Body: secret},
		{Status: 403, ContentType: "application/json", Body: secret},
		{Status: 404, ContentType: "application/json", Body: secret},
		{Status: 500, ContentType: "application/json", Body: secret},
		{Status: 203, ContentType: "text/html", Body: secret},
		{Status: 200, ContentType: "application/json", Body: strings.Repeat(secret, forge.BodyMax/len(secret)+1)},
		{Status: 200, ContentType: "application/json", Delay: 300 * time.Millisecond},
	} {
		_, err := fetch(t, f, 100*time.Millisecond)
		if err != nil && strings.Contains(err.Error(), secret) {
			t.Errorf("%d: the error carries the token: %v", f.Status, err)
		}
	}
	if strings.Contains(log.String(), secret) {
		t.Errorf("the log carries the token:\n%s", log.String())
	}
}
