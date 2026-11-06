package forge_test

import (
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
	"github.com/WilsonSousajr/omatty/internal/infra/forge"
)

// rateLimited is err's rate limit, or nil.
func rateLimited(err error) *dforge.RateLimitError {
	var limited *dforge.RateLimitError
	if errors.As(err, &limited) {
		return limited
	}
	return nil
}

var limitNow = time.Date(2026, 11, 6, 15, 0, 0, 0, time.UTC)

// A forge names its wait in one of three ways, and Bitbucket Cloud may name
// none: Retry-After in seconds or as a date, or GitHub's and GitLab's reset
// epoch. One that will not parse is no time at all, never a wait of zero.
func TestRetryAt_SecondsAndDate_issue594(t *testing.T) {
	cases := []struct {
		name   string
		header http.Header
		want   time.Time
	}{
		{"seconds", http.Header{"Retry-After": {"120"}}, limitNow.Add(2 * time.Minute)},
		{"date", http.Header{"Retry-After": {"Fri, 06 Nov 2026 15:04:00 GMT"}}, time.Date(2026, 11, 6, 15, 4, 0, 0, time.UTC)},
		{"reset epoch", http.Header{"X-Ratelimit-Reset": {strconv.FormatInt(limitNow.Add(5*time.Minute).Unix(), 10)}}, limitNow.Add(5 * time.Minute)},
		{"unreadable", http.Header{"Retry-After": {"soon"}}, time.Time{}},
		{"none", http.Header{}, time.Time{}},
	}
	for _, c := range cases {
		if got := forge.RetryAt(c.header, limitNow); !got.Equal(c.want) {
			t.Errorf("%s: RetryAt = %v, want %v", c.name, got, c.want)
		}
	}
}

// A 429 is "later" with the time the forge named, not an outage asked again
// on the next poll (#594).
func TestAnswerError_429IsARateLimit_issue594(t *testing.T) {
	f := &FakeAPI{Status: 429, ContentType: "application/json", Body: `{}`, Header: http.Header{"Retry-After": {"60"}}}
	srv := f.serve(t)
	_, err := forge.RESTGetAt(srv, srv.URL+"/2.0/repositories/w/r", forge.PrivateToken(secret), "BITBUCKET_TOKEN", limitNow)
	limited := rateLimited(err)
	if limited == nil {
		t.Fatalf("err = %v, want a RateLimitError", err)
	}
	if !limited.Until.Equal(limitNow.Add(time.Minute)) || limited.Host != srv.Listener.Addr().String() {
		t.Errorf("limit = %+v, want host %s until %v", limited, srv.Listener.Addr(), limitNow.Add(time.Minute))
	}
}

// GitHub's primary limit is a 403 with X-RateLimit-Remaining at 0. It was
// already kept from stopping the project as auth; now it waits as well.
func TestAnswerError_GitHub403RateLimitIsNotAuth_issue594(t *testing.T) {
	f := &FakeAPI{Status: 403, ContentType: "application/json", Body: `{}`, Header: http.Header{"X-Ratelimit-Remaining": {"0"}}}
	srv := f.serve(t)
	_, err := forge.RESTGetAt(srv, srv.URL+"/repos/o/r/pulls", forge.PrivateToken(secret), "GH_TOKEN", limitNow)
	var refused *dforge.AuthError
	if rateLimited(err) == nil || errors.As(err, &refused) {
		t.Errorf("err = %v, want a RateLimitError and no AuthError", err)
	}
}

// glab and tea report the status in their stderr; a 429 there is a rate
// limit with no time, since the CLI keeps the headers to itself.
func TestStatusError_CLI429_issue594(t *testing.T) {
	limited := rateLimited(forge.StatusError("gitlab.com", "glab", 429, "glab: 429 Too Many Requests (HTTP 429)"))
	if limited == nil || limited.Host != "gitlab.com" || !limited.Until.IsZero() {
		t.Errorf("limit = %+v, want gitlab.com with no time", limited)
	}
}

// A CLI's 403 that says "rate limit" is the limit, not a refused login: as an
// AuthError it stopped the project for the run.
func TestStatusError_CLI403SayingRateLimitIsNotAuth_issue594(t *testing.T) {
	err := forge.StatusError("github.com", "gh", 403, "API rate limit exceeded for user ID 1 (HTTP 403)")
	var refused *dforge.AuthError
	if rateLimited(err) == nil || errors.As(err, &refused) {
		t.Errorf("err = %v, want a RateLimitError and no AuthError", err)
	}
}

// gh's own pr list and issue list carry no status, only gh's words.
func TestClassify_GhRateLimitText_issue594(t *testing.T) {
	err := forge.Classify("/repo", "pr list", "GraphQL: API rate limit exceeded for user ID 1.", errors.New("exit status 1"))
	if limited := rateLimited(err); limited == nil || limited.Host != "github.com" {
		t.Errorf("err = %v, want a RateLimitError for github.com", err)
	}
}
