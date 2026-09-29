package forge

import (
	"bytes"
	"context"
	"net/url"
	"strconv"
)

// The bodies Azure's pull request endpoints read. Completing keeps the source
// branch, written out (#331), and sets no autoCompleteSetBy: auto-complete is
// Azure's merge-when-green, which #331 refuses by name.
type (
	azOpen struct {
		Source string `json:"sourceRefName"`
		Target string `json:"targetRefName"`
		Title  string `json:"title"`
	}
	azCommitRef struct {
		CommitID string `json:"commitId"`
	}
	azComplete struct {
		Status            string      `json:"status"`
		LastMergeSource   azCommitRef `json:"lastMergeSourceCommit"`
		CompletionOptions struct {
			DeleteSourceBranch bool `json:"deleteSourceBranch"`
		} `json:"completionOptions"`
	}
	azPolicies struct {
		Value []struct {
			IsEnabled  bool `json:"isEnabled"`
			IsBlocking bool `json:"isBlocking"`
		} `json:"value"`
	}
)

// azSend is one Azure write, decoded as azGet decodes a read.
func azSend[T any](ctx context.Context, a azBackend, method, u string, body any) (T, error) {
	return writeJSON[T](method+" "+u, body, func(b []byte) ([]byte, error) {
		return a.rest.send(ctx, method, u, bytes.NewReader(b), a.auth, a.env)
	})
}

// createPR opens a pull request for head against base (#464).
func (a azBackend) createPR(ctx context.Context, _, head, base, title string) (int, error) {
	pr, err := azSend[struct {
		ID int `json:"pullRequestId"`
	}](ctx, a, "POST", a.repoAPI("pullrequests"), azOpen{Source: "refs/heads/" + head, Target: "refs/heads/" + base, Title: title})
	if err != nil {
		return 0, err
	}
	return opened(pr.ID, a.host, "pull request")
}

// mergePR completes the pull request now. Azure completes only at the source
// commit it was last merged at, so that is read first - a push since then
// makes the completion fail rather than merge what nobody reviewed.
func (a azBackend) mergePR(ctx context.Context, _ string, number int) error {
	u := a.repoAPI("pullrequests/" + strconv.Itoa(number))
	pr, err := azGet[struct {
		LastMergeSource azCommitRef `json:"lastMergeSourceCommit"`
	}](ctx, a, u)
	if err != nil {
		return err
	}
	_, err = azSend[struct{}](ctx, a, "PATCH", u, azComplete{Status: "completed", LastMergeSource: pr.LastMergeSource})
	return err
}

// branchProtected is whether an enabled, blocking policy applies to branch -
// Azure's protection is its branch policies, and there is no flag to read.
func (a azBackend) branchProtected(ctx context.Context, _, branch string) (bool, error) {
	repo, err := azGet[struct {
		ID string `json:"id"`
	}](ctx, a, a.api("git/repositories/"+url.PathEscape(a.repo)))
	if err != nil {
		return true, err
	}
	policies, err := azGet[azPolicies](ctx, a, a.api("git/policy/configurations?repositoryId="+
		url.QueryEscape(repo.ID)+"&refName="+url.QueryEscape("refs/heads/"+branch)))
	if err != nil {
		return true, err
	}
	for _, p := range policies.Value {
		if p.IsEnabled && p.IsBlocking {
			return true, nil
		}
	}
	return false, nil
}
