package forge_test

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// scriptedAnswer answers pq from first, and reports whether it did: a
// Bitbucket fake's scripted answers.
func scriptedAnswer(w http.ResponseWriter, pq string, first []giteaAnswer) bool {
	for _, a := range first {
		if strings.Contains(pq, a.match) {
			w.WriteHeader(a.status)
			_, _ = w.Write([]byte(a.body))
			return true
		}
	}
	return false
}

// bbPRJSON is one Bitbucket pull request, as its list writes it.
func bbPRJSON(id, state, draft, author, source string) string {
	return `{"id": ` + id + `, "title": "t", "state": "` + state + `", "draft": ` + draft +
		`, "author": {"nickname": "` + author + `", "display_name": "Display Name"}, "updated_on": "2026-09-20T10:00:00Z",` +
		` "source": {"branch": {"name": "b"}, "commit": {"hash": "aaaaaaaaaaaa"}, "repository": {"full_name": "` + source + `"}},` +
		` "destination": {"repository": {"full_name": "atlassianlabs/atlascode"}}}`
}

// Only bitbucket.org is Cloud. A self-hosted host the operator named as
// bitbucket is never read through Cloud's API - its token went to
// api.bitbucket.org, and a public repository there with the same slug would
// have filled the card with a stranger's pull requests (#460's review).
func TestBitbucket_ASelfHostedHostIsNeverSentToCloud_issue460(t *testing.T) {
	api := &FakeBitbucketAPI{}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "ssh://git@bitbucket.corp.example:7999/atlassianlabs/atlascode.git"}).url,
			Hosts: forge.Hosts{"bitbucket.corp.example": forge.KindBitbucket}},
		Env: bitbucketToken, API: api.serve(t),
	})

	_, err := r.ListPRs(t.TempDir())

	for _, got := range api.Got {
		if got.Host == "api.bitbucket.org" {
			t.Errorf("sent %+v to Bitbucket Cloud for a self-hosted project", got)
		}
	}
	if err == nil {
		t.Error("ListPRs = nil error, want a self-hosted project not read as Cloud")
	}
}

// With a token, Bitbucket's 404 is most often a token that cannot see the
// repository - its own message says "make sure you are authenticated" - so it
// is a note naming the token, not "this project is not on Bitbucket" (#460's
// review).
func TestBitbucket_A404WithATokenNamesTheToken_issue460(t *testing.T) {
	_, err := bitbucketRouter(t, bitbucketToken, &FakeBitbucketAPI{Status: 404}).ListPRs(t.TempDir())

	var refused *forge.AuthError
	if !errors.As(err, &refused) || refused.TokenEnv != "BITBUCKET_TOKEN" || refused.Status != 404 {
		t.Errorf("error = %v, want BITBUCKET_TOKEN named with the 404", err)
	}
}

// An API token is Basic auth with the account as the user, in that order.
func TestBitbucket_BasicAuthIsUserThenToken_issue460(t *testing.T) {
	api := &FakeBitbucketAPI{}
	_, _ = bitbucketRouter(t, bitbucketToken, api).ListPRs(t.TempDir())

	if want := "Basic " + base64.StdEncoding.EncodeToString([]byte("someone:"+secret)); len(api.Got) == 0 || api.Got[0].Auth != want {
		t.Errorf("sent %+v, want %s", api.Got, want)
	}
}

// Page two is asked for only after a full page one: Bitbucket counts every
// request against an hourly limit, and asking both at once doubled the cost
// of every list (#460's review).
func TestBitbucket_AShortPageIsTheLast_issue460(t *testing.T) {
	api := &FakeBitbucketAPI{}
	_, _ = bitbucketRouter(t, bitbucketToken, api).ListPRs(t.TempDir())

	open := 0
	for _, got := range api.Got {
		if strings.Contains(got.Path, "state=OPEN") {
			open++
		}
	}
	if open != 1 {
		t.Errorf("asked for the open list %d times, want once for a short page", open)
	}
}

// The fold, field by field, from answers that carry each case: a draft, a
// fork, a declined and a superseded one closed, a merged one's time, the
// finished list's own request, and a user with no nickname (#460's review:
// each could be broken with every test green).
func TestBitbucket_FoldsEveryCase_issue460(t *testing.T) {
	open := `{"values": [` + bbPRJSON("1", "OPEN", "true", "ray", "atlassianlabs/atlascode") + `,` +
		bbPRJSON("2", "OPEN", "false", "", "someone/atlascode-fork") + `]}`
	done := `{"values": [` + bbPRJSON("3", "MERGED", "false", "ray", "atlassianlabs/atlascode") + `,` +
		bbPRJSON("4", "SUPERSEDED", "false", "ray", "atlassianlabs/atlascode") + `]}`
	api := &FakeBitbucketAPI{First: []giteaAnswer{
		{match: "state=OPEN", status: 200, body: open}, {match: "state=MERGED&state=DECLINED", status: 200, body: done},
		{match: "/statuses", status: 200, body: `{"values": []}`},
	}}
	prs, err := bitbucketRouter(t, bitbucketToken, api).ListPRs(t.TempDir())
	one, _ := prNumbered(prs, 1)
	two, _ := prNumbered(prs, 2)
	three, _ := prNumbered(prs, 3)
	four, _ := prNumbered(prs, 4)
	if err != nil || !one.Draft || one.Fork || !two.Fork || three.State != forge.Merged || three.MergedAt.IsZero() || four.State != forge.Closed {
		t.Errorf("ListPRs = %+v, %v; want a draft, a fork, merged with its time and superseded closed", prs, err)
	}
}

// A finished list that fails is not a card with nothing finished.
func TestBitbucket_AFinishedListFailureIsNotHidden_issue460(t *testing.T) {
	api := &FakeBitbucketAPI{First: []giteaAnswer{{match: "state=MERGED", status: 500, body: `{}`}}}

	if _, err := bitbucketRouter(t, bitbucketToken, api).ListPRs(t.TempDir()); err == nil {
		t.Error("ListPRs = nil error, want the finished list's failure")
	}
}

// The CI mark is the worst of the head's statuses, read for that head.
func TestBitbucket_CIIsTheWorstStatus_issue460(t *testing.T) {
	statuses := `{"values": [{"state": "SUCCESSFUL", "name": "a"}, {"state": "FAILED", "name": "b"}, {"state": "SUCCESSFUL", "name": "c"}]}`
	api := &FakeBitbucketAPI{First: []giteaAnswer{{match: "/commit/31b8ff8dad0a/statuses", status: 200, body: statuses}}}

	prs, err := bitbucketRouter(t, bitbucketToken, api).ListPRs(t.TempDir())

	if pr, _ := prNumbered(prs, 1115); err != nil || pr.CI != forge.CIFailing {
		t.Errorf("#1115 CI = %v, %v; want failing", pr.CI, err)
	}
}

// An item's comments: a deleted one is left out, and a comment list that
// fails or has another page marks the item cut (#397; #460's review).
func TestBitbucket_CommentsAreWholeOrMarkedCut_issue460(t *testing.T) {
	comments := `{"values": [{"content": {"raw": "kept"}, "user": {"nickname": "a"}}, {"content": {"raw": "gone"}, "user": {"nickname": "b"}, "deleted": true}]}`
	for name, c := range map[string]struct {
		answer giteaAnswer
		cut    bool
		bodies string
	}{
		"whole":     {giteaAnswer{match: "/comments", status: 200, body: comments}, false, "kept"},
		"next page": {giteaAnswer{match: "/comments", status: 200, body: strings.Replace(comments, `{"values"`, `{"next": "https://api.bitbucket.org/2.0/x?page=2", "values"`, 1)}, true, "kept"},
		"failed":    {giteaAnswer{match: "/comments", status: 500, body: `{}`}, true, ""},
	} {
		d, err := bitbucketRouter(t, bitbucketToken, &FakeBitbucketAPI{First: []giteaAnswer{c.answer}}).ViewPR(t.TempDir(), 1115)
		var got []string
		for _, cm := range d.Comments {
			got = append(got, cm.Body)
		}
		if err != nil || d.Truncated != c.cut || strings.Join(got, ",") != c.bodies {
			t.Errorf("%s: comments %q, cut %v, %v; want %q, cut %v", name, got, d.Truncated, err, c.bodies, c.cut)
		}
	}
}

// Bitbucket keeps no issues, so there is no issue page to open either.
func TestBitbucket_BrowseHasNoIssuePage_issue460(t *testing.T) {
	if err := bitbucketRouter(t, bitbucketToken, &FakeBitbucketAPI{}).BrowseIssue(t.TempDir(), 1); !errors.Is(err, forge.ErrNoTracker) {
		t.Errorf("BrowseIssue = %v, want ErrNoTracker", err)
	}
}

// A user is their nickname, or, with none, their display name.
func TestBitbucket_AUserWithNoNicknameIsTheirName_issue460(t *testing.T) {
	comments := `{"values": [{"content": {"raw": "hi"}, "user": {"display_name": "Only Display"}}]}`
	api := &FakeBitbucketAPI{First: []giteaAnswer{{match: "/comments", status: 200, body: comments}}}

	d, err := bitbucketRouter(t, bitbucketToken, api).ViewPR(t.TempDir(), 1115)

	if err != nil || len(d.Comments) != 1 || d.Comments[0].Author != "Only Display" {
		t.Errorf("comments = %+v, %v; want the display name", d.Comments, err)
	}
}
