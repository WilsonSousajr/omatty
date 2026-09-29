package forge_test

import (
	"encoding/base64"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// cloudAndDC is one Router over a Cloud project and a Data Center one, as an
// operator with both would have, with env and every request sent to api.
func cloudAndDC(t *testing.T, env map[string]string, api *FakeBitbucketDCAPI) (*forge.Router, string, string) {
	cloud, dc := t.TempDir(), t.TempDir()
	remotes := map[string]string{cloud: "https://bitbucket.org/ws/app.git", dc: "https://git.corp.example/scm/ops/platform.git"}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{
			Remote: func(root string) (string, error) { return remotes[root], nil },
			Hosts:  forge.Hosts{"git.corp.example": forge.KindBitbucket},
		},
		Env: env, API: api.serve(t),
	})
	return r, cloud, dc
}

// A Bitbucket token is sent only where it is for: BITBUCKET_TOKEN to Cloud,
// BITBUCKET_DC_TOKEN to the instance BITBUCKET_DC_URL names. An Atlassian API
// token covers the whole account - Jira and Confluence too - and it was going
// to every Data Center host the operator named, and a Data Center token to
// Cloud (#461's review).
func TestBitbucket_ATokenGoesOnlyWhereItIsFor_issue461(t *testing.T) {
	cloudAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("me@corp:"+secret))
	api := &FakeBitbucketDCAPI{}
	r, cloud, dc := cloudAndDC(t, map[string]string{"BITBUCKET_TOKEN": secret, "BITBUCKET_USER": "me@corp"}, api)
	_, _ = r.ListPRs(cloud)
	_, err := r.ListPRs(dc)

	var missing *forge.MissingToolError
	if !errors.As(err, &missing) || !strings.HasPrefix(missing.TokenEnv, "BITBUCKET_DC_TOKEN") {
		t.Errorf("Data Center with only a Cloud token: %v, want BITBUCKET_DC_TOKEN unset", err)
	}
	for _, got := range api.Got {
		if got.Host != "api.bitbucket.org" && got.Auth != "" {
			t.Errorf("sent %q to %s, want Cloud's token to Cloud alone", got.Auth, got.Host)
		}
		if got.Host == "api.bitbucket.org" && got.Auth != cloudAuth {
			t.Errorf("sent %q to Cloud, want its own Basic auth", got.Auth)
		}
	}
}

// A Data Center token set for another instance is not sent to this one, and
// BITBUCKET_DC_USER makes it Basic auth, as a Cloud user does.
func TestBitbucketDC_ATokenForAnotherInstanceStaysHome_issue461(t *testing.T) {
	other := map[string]string{"BITBUCKET_DC_TOKEN": secret, "BITBUCKET_DC_URL": "https://git.other.example"}
	api := &FakeBitbucketDCAPI{}
	_, err := dcRouter(t, "https://git.corp.example/scm/ops/platform.git", other, api).ListPRs(t.TempDir())
	var missing *forge.MissingToolError
	if !errors.As(err, &missing) || len(api.Got) != 0 {
		t.Errorf("another instance's token: %v after %+v, want nothing sent and a note", err, api.Got)
	}
	withUser := map[string]string{"BITBUCKET_DC_TOKEN": secret, "BITBUCKET_DC_URL": "https://git.corp.example", "BITBUCKET_DC_USER": "jdoe"}
	api = &FakeBitbucketDCAPI{}
	_, _ = dcRouter(t, "https://git.corp.example/scm/ops/platform.git", withUser, api).ListPRs(t.TempDir())
	if want := "Basic " + base64.StdEncoding.EncodeToString([]byte("jdoe:"+secret)); len(api.Got) == 0 || api.Got[0].Auth != want {
		t.Errorf("sent %+v, want Basic auth for jdoe", api.Got)
	}
}

// An ssh clone carries no web root - no context path, no web port - so
// BITBUCKET_DC_URL gives it: the instance at https://host/bitbucket is asked
// there, not at https://host, where it would read as no forge (#461's review).
func TestBitbucketDC_AnSSHCloneReadsTheInstancesWebRoot_issue461(t *testing.T) {
	env := map[string]string{"BITBUCKET_DC_TOKEN": secret, "BITBUCKET_DC_URL": "https://git.corp.example/bitbucket"}
	api := &FakeBitbucketDCAPI{}

	_, err := dcRouter(t, "ssh://git@git.corp.example:7999/ops/platform.git", env, api).ListPRs(t.TempDir())

	if err != nil || len(api.Got) == 0 || !strings.HasPrefix(api.Got[0].Path, "/bitbucket/rest/api/1.0/projects/ops/repos/platform/") {
		t.Errorf("ListPRs = %v, sent %+v; want the context path from BITBUCKET_DC_URL", err, api.Got)
	}
}

// An http instance is refused before any token question: the note that asked
// for a token first led only to the https note (#461's review).
func TestBitbucketDC_AnHTTPInstanceSaysHTTPSFirst_issue461(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"no token":         nil,
		"an http instance": {"BITBUCKET_DC_TOKEN": secret, "BITBUCKET_DC_URL": "http://git.corp.example:7990"},
	} {
		remote := "http://git.corp.example/scm/ops/platform.git"
		if env != nil {
			remote = "ssh://git@git.corp.example:7999/ops/platform.git"
		}
		api := &FakeBitbucketDCAPI{}
		_, err := dcRouter(t, remote, env, api).ListPRs(t.TempDir())
		var plain *forge.PlainHTTPError
		if !errors.As(err, &plain) || len(api.Got) != 0 {
			t.Errorf("%s: %v after %+v, want the https-only note and nothing sent", name, err, api.Got)
		}
	}
}

// Data Center's build states: CANCELLED is a check that did not pass, as
// Cloud's STOPPED is; UNKNOWN is no result - neither is running forever.
func TestBitbucket_BuildStatesAreFinal_issue461(t *testing.T) {
	for state, want := range map[string]forge.CIState{"CANCELLED": forge.CIFailing, "UNKNOWN": forge.CINone} {
		if got := forge.BitbucketCI(state); got != want {
			t.Errorf("%s = %v, want %v", state, got, want)
		}
	}
}

// A pull request's comments are the thread as written: replies nested under
// their comment included, an edit or a deletion not counted as a comment, a
// deleted one left out, in the order they were made; and a feed with more
// pages marks the item cut (#461's review).
func TestBitbucketDC_ReadsTheCommentThread_issue461(t *testing.T) {
	feed := `{"isLastPage": false, "values": [
	 {"action": "COMMENTED", "commentAction": "EDITED", "comment": {"id": 9, "text": "edited", "author": {"displayName": "B"}, "createdDate": 1790300000000}},
	 {"action": "COMMENTED", "commentAction": "DELETED", "comment": {"id": 8, "text": "gone", "author": {"displayName": "C"}, "createdDate": 1790250000000}},
	 {"action": "COMMENTED", "commentAction": "ADDED", "comment": {"id": 8, "text": "gone", "author": {"displayName": "C"}, "createdDate": 1790250000000}},
	 {"action": "COMMENTED", "commentAction": "ADDED", "comment": {"id": 9, "text": "edited", "author": {"displayName": "B"}, "createdDate": 1790200000000,
	   "comments": [{"id": 10, "text": "a reply", "author": {"displayName": "A"}, "createdDate": 1790210000000}]}},
	 {"action": "COMMENTED", "commentAction": "ADDED", "comment": {"id": 7, "text": "first", "author": {"displayName": "A"}, "createdDate": 1790100000000}}
	]}`
	api := &FakeBitbucketDCAPI{First: []giteaAnswer{{match: "/activities", status: 200, body: feed}}}

	d, err := dcRouter(t, "https://git.corp.example/scm/ops/platform.git", dcToken, api).ViewPR(t.TempDir(), 42)

	var got []string
	for _, c := range d.Comments {
		got = append(got, c.Body)
	}
	if want := []string{"first", "edited", "a reply"}; err != nil || !reflect.DeepEqual(got, want) || !d.Truncated {
		t.Errorf("ViewPR = %q, truncated %v, %v; want %q and cut", got, d.Truncated, err, want)
	}
}

// Every field the fold reads, exactly: dates are epoch milliseconds, a merged
// one's time is its close, the declined are asked for, and the item carries
// its creation, its page and each build's own state (#461's review: each of
// these could be broken with every test green).
func TestBitbucketDC_FoldsEveryFieldExactly_issue461(t *testing.T) {
	api := &FakeBitbucketDCAPI{}
	r := dcRouter(t, "https://git.corp.example/scm/ops/platform.git", dcToken, api)
	prs, err := r.ListPRs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	open, _ := prNumbered(prs, 42)
	merged, _ := prNumbered(prs, 40)
	declined, found := prNumbered(prs, 39)
	if !open.Updated.Equal(time.UnixMilli(1790500000000)) || !merged.MergedAt.Equal(time.UnixMilli(1788500000000)) || !found || declined.State != forge.Closed {
		t.Errorf("updated %v, merged at %v, declined %+v; want the fixture's milliseconds and the declined read", open.Updated, merged.MergedAt, declined)
	}
	d, _ := r.ViewPR(t.TempDir(), 42)
	states := []forge.CIState{}
	for _, c := range d.Checks {
		states = append(states, c.State)
	}
	if !d.Created.Equal(time.UnixMilli(1790000000000)) || d.URL != "https://git.corp.example/projects/OPS/repos/platform/pull-requests/42" ||
		!reflect.DeepEqual(states, []forge.CIState{forge.CIPassing, forge.CIFailing}) {
		t.Errorf("item created %v at %q with checks %v; want the fixture's", d.Created, d.URL, states)
	}
}

// The page b opens keeps the instance's context path.
func TestBitbucketDC_BrowseKeepsTheContextPath_issue461(t *testing.T) {
	var opened []string
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "https://git.corp.example/bitbucket/scm/ops/platform.git"}).url,
			Hosts: forge.Hosts{"git.corp.example": forge.KindBitbucket}},
		Env: dcToken, API: (&FakeBitbucketDCAPI{}).serve(t),
		Open: func(u string) error { opened = append(opened, u); return nil },
	})

	_ = r.BrowsePR(t.TempDir(), 42)

	if want := []string{"https://git.corp.example/bitbucket/projects/ops/repos/platform/pull-requests/42"}; !reflect.DeepEqual(opened, want) {
		t.Errorf("opened %v, want %v", opened, want)
	}
}

// An ssh clone's path has no /scm/ prefix, so a project keyed SCM is a
// project, not a prefix to strip (#461's review).
func TestBitbucketDC_AnSSHProjectKeyedSCMIsAProject_issue461(t *testing.T) {
	env := map[string]string{"BITBUCKET_DC_TOKEN": secret, "BITBUCKET_DC_URL": "https://git.corp.example"}
	api := &FakeBitbucketDCAPI{}

	_, _ = dcRouter(t, "ssh://git@git.corp.example:7999/scm/tools.git", env, api).ListPRs(t.TempDir())

	if len(api.Got) == 0 || !strings.HasPrefix(api.Got[0].Path, "/rest/api/1.0/projects/scm/repos/tools/") {
		t.Errorf("sent %+v, want project scm, repository tools", api.Got)
	}
}

// Cloud's ssh over port 443 is Cloud, as GitHub's and GitLab's are theirs.
func TestBitbucket_AltSSHIsCloud_issue461(t *testing.T) {
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "ssh://git@altssh.bitbucket.org:443/ws/app.git"}).url,
			Hosts: forge.Hosts{"altssh.bitbucket.org": forge.KindBitbucket}},
	})
	root := t.TempDir()
	_, _ = r.ListPRs(root)

	if l := r.Label(root); l.Forge != "Bitbucket" {
		t.Errorf("label = %+v, want Bitbucket Cloud's", l)
	}
	var missing *forge.MissingToolError
	if _, err := r.ListPRs(root); !errors.As(err, &missing) || missing.TokenEnv != "BITBUCKET_TOKEN" {
		t.Errorf("error = %v, want Cloud's BITBUCKET_TOKEN named", err)
	}
}
