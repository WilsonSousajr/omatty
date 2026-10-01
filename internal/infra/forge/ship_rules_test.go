package forge_test

import (
	"errors"
	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
	"strings"
	"testing"
)

// shipForgeNamed is one row of shipForges, with routes added over its own.
func shipForgeNamed(t *testing.T, name string, routes map[string]string) (shipForge, *FakeWriteAPI) {
	t.Helper()
	for _, s := range shipForges {
		if s.name != name {
			continue
		}
		all := map[string]string{}
		for k, v := range s.routes {
			all[k] = v
		}
		for k, v := range routes {
			all[k] = v
		}
		return s, &FakeWriteAPI{Routes: all}
	}
	t.Fatalf("no ship forge named %q", name)
	return shipForge{}, nil
}

// protectedBy asks one forge whether main is protected, given answer to its
// protection read.
func protectedBy(t *testing.T, name, answer string) (bool, error) {
	s, _ := shipForgeNamed(t, name, nil)
	_, api := shipForgeNamed(t, name, map[string]string{"GET " + s.protect: answer})
	return s.router(t, api).BranchProtected(t.TempDir(), "main")
}

// Bitbucket has no per-branch flag: its restrictions are matched here. A glob
// covers what it matches, "?" one character and nothing else special; a
// branching-model restriction, and a page left unread, are protected - the
// side #331 fails to (#464).
func TestShip_BitbucketRestrictionsAreMatched_issue464(t *testing.T) {
	for answer, want := range map[string]bool{
		`{"values": [{"branch_match_kind": "glob", "pattern": "m?in"}]}`:                      true,
		`{"values": [{"branch_match_kind": "glob", "pattern": "ma.n"}]}`:                      false,
		`{"values": [{"branch_match_kind": "glob", "pattern": "*"}]}`:                         true,
		`{"values": [{"branch_match_kind": "branching_model", "branch_type": "production"}]}`: true,
		`{"values": [], "next": "https://api.bitbucket.org/2.0/page2"}`:                       true,
		`{"values": []}`: false,
	} {
		if got, err := protectedBy(t, "Bitbucket", answer); err != nil || got != want {
			t.Errorf("%s: protected = %v, %v; want %v", answer, got, err, want)
		}
	}
}

// Data Center's permissions name a branch, a pattern, or a branching-model
// matcher only the model resolves; an unread page is protected.
func TestShip_DataCenterPermissionsAreMatched_issue464(t *testing.T) {
	for answer, want := range map[string]bool{
		`{"isLastPage": true, "values": [{"matcher": {"id": "main", "type": {"id": "BRANCH"}}}]}`:               true,
		`{"isLastPage": true, "values": [{"matcher": {"id": "refs/heads/ma*", "type": {"id": "PATTERN"}}}]}`:    true,
		`{"isLastPage": true, "values": [{"matcher": {"id": "release/*", "type": {"id": "PATTERN"}}}]}`:         false,
		`{"isLastPage": true, "values": [{"matcher": {"id": "production", "type": {"id": "MODEL_CATEGORY"}}}]}`: true,
		`{"isLastPage": false, "values": []}`: true,
	} {
		if got, err := protectedBy(t, "Bitbucket Data Center", answer); err != nil || got != want {
			t.Errorf("%s: protected = %v, %v; want %v", answer, got, err, want)
		}
	}
}

// A branch answer without its protected flag is not "unprotected": it is
// protected beside the reason (#331 fails closed).
func TestShip_AnAnswerWithoutTheFlagIsProtected_issue464(t *testing.T) {
	for _, name := range []string{"GitLab", "Gitea", "GitHub"} {
		if got, err := protectedBy(t, name, `{"name": "main"}`); !got || err == nil {
			t.Errorf("%s: protected = %v, %v; want protected beside an error", name, got, err)
		}
	}
}

// GitHub's merge uses the repository's own method, the first its merge button
// offers; a repository that allows none this token can see is refused.
func TestShip_GitHubMergesWithTheRepositorysMethod_issue464(t *testing.T) {
	for answer, want := range map[string]string{
		`{"allow_merge_commit": true, "allow_squash_merge": true}`:   `"merge_method":"merge"`,
		`{"allow_rebase_merge": true}`:                               `"merge_method":"rebase"`,
		`{"allow_merge_commit": false, "allow_squash_merge": false}`: "",
	} {
		s, api := shipForgeNamed(t, "GitHub", map[string]string{"GET /repos/owner/app?": answer})
		_, err := s.router(t, api).MergePR(t.TempDir(), 17, greenHead)
		w, merged := api.write("PUT", "/pulls/17/merge")
		switch {
		case want == "" && (err == nil || merged):
			t.Errorf("%s: merged (%v), want a refusal", answer, err)
		case want != "" && (err != nil || !strings.Contains(asJSON(w.Body), want)):
			t.Errorf("%s: MergePR = %v, sent %+v; want %s", answer, err, w, want)
		}
	}
}

// Gitea's merge has no default of its own; a repository older than the
// setting merges with a merge commit.
func TestShip_GiteaWithNoDefaultStyleMerges_issue464(t *testing.T) {
	s, api := shipForgeNamed(t, "Gitea", map[string]string{"GET /repos/owner/app?": `{}`})

	_, err := s.router(t, api).MergePR(t.TempDir(), 17, greenHead)

	if w, _ := api.write("POST", "/pulls/17/merge"); err != nil || !strings.Contains(asJSON(w.Body), `"Do":"merge"`) {
		t.Errorf("MergePR = %v, sent %+v; want Do merge", err, w)
	}
}

// A forge that opens a change and names no number has opened nothing the
// card could find: an error, never change 0.
func TestShip_AnOpenWithNoNumberIsAnError_issue464(t *testing.T) {
	for _, s := range shipForges {
		api := &FakeWriteAPI{Routes: map[string]string{s.open[0] + " " + s.open[1]: `{}`}}
		if n, err := s.router(t, api).CreatePR(t.TempDir(), "feat/parser", "main", "t"); err == nil || n != 0 {
			t.Errorf("%s: CreatePR = %d, %v; want an error", s.name, n, err)
		}
	}
}

// Regression, #599: a merge names the head the card showed green, so a push
// after the last poll is never merged unread. Bitbucket, which takes no head,
// is asked for the pull request's own first and refuses when it moved.
func TestShip_AHeadThatMovedIsNotMerged_issue599(t *testing.T) {
	for name, get := range map[string][2]string{
		"Bitbucket":             {"GET /pullrequests/17", `{"id": 17, "source": {"commit": {"hash": "deadbeefdead"}}}`},
		"Bitbucket Data Center": {"GET /pull-requests/17", `{"id": 17, "version": 3, "fromRef": {"latestCommit": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}}`},
	} {
		s, api := shipForgeNamed(t, name, map[string]string{get[0]: get[1]})
		_, err := s.router(t, api).MergePR(t.TempDir(), 17, greenHead)
		if _, merged := api.write(s.merge[0], s.merge[1]); err == nil || merged {
			t.Errorf("%s: MergePR = %v, merge sent %v; want a refusal and no merge", name, err, merged)
		}
	}
}

// A merge the forge has only accepted - Azure completes asynchronously,
// Bitbucket answers 202 past its timeout - is not reported as merged (#464's
// review): the card would read "merged" for a merge that could still fail.
func TestShip_AMergeOnlyAcceptedIsNotMerged_issue464(t *testing.T) {
	for name, route := range map[string][2]string{
		"Azure DevOps": {"PATCH /pullrequests/17", `{"status": "active", "mergeStatus": "queued"}`},
		"GitLab":       {"PUT /merge_requests/17/merge", `{"state": "opened", "merge_status": "checking"}`},
	} {
		s, api := shipForgeNamed(t, name, map[string]string{route[0]: route[1]})
		if merged, err := s.router(t, api).MergePR(t.TempDir(), 17, greenHead); err != nil || merged {
			t.Errorf("%s: MergePR = %v, %v; want accepted, not merged", name, merged, err)
		}
	}
}

// Data Center matches a branch permission's pattern the Ant way, against a
// suffix of refs/heads/<branch>: `**` spans segments, a trailing `/` covers
// everything under it, `*` stays within one segment. Matched anchored, each of
// these read as unprotected and would have been merged into (#464's review).
func TestShip_DataCenterPatternsMatchAsDataCenterDoes_issue464(t *testing.T) {
	for _, c := range []struct {
		pattern, branch string
		protected       bool
	}{
		{"heads/**/master", "master", true},
		{"PROJECT-*", "stable/PROJECT-new", true},
		{"release/", "release/2.0", true},
		{"heads/release/*", "release/1", true},
		{"develop", "team/develop", true},
		{"release/*", "main", false},
		{"release/*", "release/1/2", false},
	} {
		answer := `{"isLastPage": true, "values": [{"matcher": {"id": "` + c.pattern + `", "type": {"id": "PATTERN"}}}]}`
		s, _ := shipForgeNamed(t, "Bitbucket Data Center", nil)
		_, api := shipForgeNamed(t, "Bitbucket Data Center", map[string]string{"GET " + s.protect: answer})
		got, err := s.router(t, api).BranchProtected(t.TempDir(), c.branch)
		if err != nil || got != c.protected {
			t.Errorf("%q on %q: protected = %v, %v; want %v", c.pattern, c.branch, got, err, c.protected)
		}
	}
}

// Reading Bitbucket's branch restrictions needs repository admin: a token
// without it is refused, and the refusal says so rather than asking for a
// token that works everywhere else (#464's review).
func TestShip_BitbucketProtectionNamesTheAdminItNeeds_issue464(t *testing.T) {
	for _, name := range []string{"Bitbucket", "Bitbucket Data Center"} {
		s, _ := shipForgeNamed(t, name, nil)
		_, api := shipForgeNamed(t, name, map[string]string{"GET " + s.protect: "!403 {}"})
		got, err := s.router(t, api).BranchProtected(t.TempDir(), "main")
		if !got || err == nil || !strings.Contains(err.Error(), "admin") {
			t.Errorf("%s: protected = %v, %v; want protected, and the admin it needs named", name, got, err)
		}
	}
}

// Azure's protection is a blocking policy scoped to the branch - or to the
// default branch - and a repository-wide setting, a file size limit, gates no
// merge; the read names the repository and the ref (#464's review).
func TestShip_AzureProtectionIsABranchPolicy_issue464(t *testing.T) {
	for answer, want := range map[string]bool{
		`{"value": [{"isEnabled": true, "isBlocking": true, "settings": {"scope": [{"repositoryId": "repo-guid"}]}}]}`:                     false,
		`{"value": [{"isEnabled": true, "isBlocking": false, "settings": {"scope": [{"refName": "refs/heads/main"}]}}]}`:                   false,
		`{"value": [{"isEnabled": true, "isBlocking": true, "settings": {"scope": [{"refName": null, "matchKind": "DefaultBranch"}]}}]}`:   true,
		`{"value": [{"isEnabled": true, "isBlocking": true, "settings": {"scope": [{"refName": "refs/heads/", "matchKind": "Prefix"}]}}]}`: true,
	} {
		s, _ := shipForgeNamed(t, "Azure DevOps", nil)
		_, api := shipForgeNamed(t, "Azure DevOps", map[string]string{"GET " + s.protect: answer})
		got, err := s.router(t, api).BranchProtected(t.TempDir(), "main")
		if err != nil || got != want {
			t.Errorf("%s: protected = %v, %v; want %v", answer, got, err, want)
		}
		if w, ok := api.write("GET", "/policy/configurations"); !ok || !strings.Contains(w.Path, "repositoryId=repo-guid") || !strings.Contains(w.Path, "refName=refs%2Fheads%2Fmain") {
			t.Errorf("asked %+v, want the repository and the ref named", w)
		}
	}
}

// A branch name is escaped in a path: release#2 is not release.
func TestShip_ABranchNameIsEscapedInItsPath_issue464(t *testing.T) {
	for _, name := range []string{"GitHub", "Gitea"} {
		s, api := shipForgeNamed(t, name, nil)
		_, _ = s.router(t, api).BranchProtected(t.TempDir(), "release#2")
		if w, ok := api.write("GET", "/branches/release%232"); !ok {
			t.Errorf("%s: asked %+v, want /branches/release%%232", name, w)
		}
	}
}

// An anonymous Gitea read cannot open or merge anything: it says so before
// sending, with the note that names what is missing (#464's review).
func TestShip_AnAnonymousGiteaDoesNotWrite_issue464(t *testing.T) {
	s, api := shipForgeNamed(t, "Gitea", nil)
	s.env = nil
	_, err := s.router(t, api).CreatePR(t.TempDir(), "feat/parser", "main", "t")

	var missing *dforge.MissingToolError
	if !errors.As(err, &missing) || len(api.Got) != 0 {
		t.Errorf("CreatePR = %v after %+v, want the token note and nothing sent", err, api.Got)
	}
}
