package forge_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/infra/forge"
)

// FakeBitbucketCost is Bitbucket Cloud with ten open pull requests on every
// repository and no CI at all, counting what each poll costs (#594).
type FakeBitbucketCost struct {
	mu    sync.Mutex
	count int
}

func (f *FakeBitbucketCost) serve(t *testing.T) string {
	t.Helper()
	open := make([]string, 10)
	for i := range open {
		open[i] = bbPRJSON(strconv.Itoa(i+1), "OPEN", "false", "someone", "atlassianlabs/atlascode")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.count++
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.RawQuery, "state=OPEN") {
			_, _ = w.Write([]byte(`{"values": [` + strings.Join(open, ",") + `]}`))
			return
		}
		_, _ = w.Write([]byte(`{"values": []}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func (f *FakeBitbucketCost) requests() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.count
}

// #594's arithmetic, run: two repositories with no CI and ten open pull
// requests each, polled every minute for an hour, stay under Atlassian's
// 1,000 requests an hour - before, each cost about 720. Between a CI expiry
// and a finished re-read, a poll is one list read.
func TestBitbucket_NoCIProjectStaysUnderTheHourlyLimit_issue594(t *testing.T) {
	api := &FakeBitbucketCost{}
	clock := time.Date(2026, 11, 6, 15, 0, 0, 0, time.UTC)
	remotes := map[string]string{"/a": "git@bitbucket.org:w/a.git", "/b": "git@bitbucket.org:w/b.git"}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: func(root string) (string, error) { return remotes[root], nil }, Now: func() time.Time { return clock }},
		Env:     bitbucketToken, API: api.serve(t),
	})
	for poll := 1; poll <= 60; poll++ {
		before := api.requests()
		if _, err := r.ListPRs("/a"); err != nil {
			t.Fatal(err)
		}
		if cost := api.requests() - before; (poll >= 2 && poll <= 5 || poll >= 7 && poll <= 10) && cost != 1 {
			t.Errorf("poll %d cost %d requests, want 1: the open list alone", poll, cost)
		}
		if _, err := r.ListPRs("/b"); err != nil {
			t.Fatal(err)
		}
		clock = clock.Add(time.Minute)
	}
	if got := api.requests(); got >= 1000 {
		t.Errorf("an hour of two projects = %d requests, want under 1000", got)
	}
}
