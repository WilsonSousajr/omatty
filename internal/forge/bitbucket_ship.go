package forge

import (
	"context"
	"strconv"
)

// The bodies Bitbucket Cloud's pull request endpoints read. The source
// branch is kept, written out rather than left to the repository's default
// (#331); the merge strategy is left to the repository, which is its default.
type (
	bbBranchRef struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
	}
	bbOpen struct {
		Title       string      `json:"title"`
		Source      bbBranchRef `json:"source"`
		Destination bbBranchRef `json:"destination"`
		CloseSource bool        `json:"close_source_branch"`
	}
	bbMerge struct {
		CloseSource bool `json:"close_source_branch"`
	}
	// bbRestrictions is one page of branch restrictions: a glob, or a
	// branching-model kind ("production") that only the model can resolve.
	bbRestrictions struct {
		Values []struct {
			MatchKind string `json:"branch_match_kind"`
			Pattern   string `json:"pattern"`
		} `json:"values"`
		Next string `json:"next"`
	}
)

func bbBranch(name string) bbBranchRef {
	var r bbBranchRef
	r.Branch.Name = name
	return r
}

// createPR opens a pull request for head against base (#464).
func (b bbBackend) createPR(ctx context.Context, _, head, base, title string) (int, error) {
	pr, err := sendJSON[struct {
		ID int `json:"id"`
	}](ctx, b.f, "POST", b.repo()+"/pullrequests", bbOpen{Title: title, Source: bbBranch(head), Destination: bbBranch(base)})
	if err != nil {
		return 0, err
	}
	return opened(pr.ID, b.remote.Host, "pull request")
}

// mergePR merges now, with the repository's default strategy, if the pull
// request's head is still the one that was green: Bitbucket's merge takes no
// head, so it is read first (#599). A 202 - a merge Bitbucket is still doing
// past its timeout - answers no state, so it is not reported merged.
func (b bbBackend) mergePR(ctx context.Context, _ string, number int, head string) (bool, error) {
	path := b.repo() + "/pullrequests/" + strconv.Itoa(number)
	pr, err := getJSON[bbPR](ctx, b.f, path)
	if err != nil {
		return false, err
	}
	if err := stillAt(number, pr.Source.Commit.Hash, head); err != nil {
		return false, err
	}
	got, err := sendJSON[struct {
		State string `json:"state"`
	}](ctx, b.f, "POST", path+"/merge", bbMerge{})
	return got.State == "MERGED", err
}

// branchProtected is whether any restriction covers branch. Bitbucket has no
// per-branch flag, so the restrictions are read and matched here - failing
// closed on one only the branching model could resolve, and on a page left
// unread.
func (b bbBackend) branchProtected(ctx context.Context, _, branch string) (bool, error) {
	page, err := getJSON[bbRestrictions](ctx, b.f, b.repo()+"/branch-restrictions?pagelen=100")
	if err != nil {
		return true, err
	}
	return page.cover(branch), nil
}

func (p bbRestrictions) cover(branch string) bool {
	for _, r := range p.Values {
		if r.MatchKind != "glob" || globMatches(r.Pattern, branch) {
			return true
		}
	}
	return p.Next != ""
}
