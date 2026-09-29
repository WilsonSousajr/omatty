package forge_test

import (
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
