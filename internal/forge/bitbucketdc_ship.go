package forge

import (
	"context"
	"strconv"
)

// The bodies Bitbucket Data Center's pull request endpoints read. Its merge
// deletes no branch and has no merge-when-green of its own, so there is
// nothing to write out as "no" there (#331).
type (
	dcProjectRef struct {
		Key string `json:"key"`
	}
	dcRepoRef struct {
		Slug    string       `json:"slug"`
		Project dcProjectRef `json:"project"`
	}
	dcBranchRef struct {
		ID         string    `json:"id"`
		Repository dcRepoRef `json:"repository"`
	}
	dcOpen struct {
		Title   string      `json:"title"`
		FromRef dcBranchRef `json:"fromRef"`
		ToRef   dcBranchRef `json:"toRef"`
	}
	// dcRestrictions is one page of branch permissions: each names what it
	// matches - a branch, a pattern, or a branching-model category or branch
	// that only the model can resolve.
	dcRestrictions struct {
		Values []struct {
			Matcher struct {
				ID   string `json:"id"`
				Type struct {
					ID string `json:"id"`
				} `json:"type"`
			} `json:"matcher"`
		} `json:"values"`
		IsLastPage bool `json:"isLastPage"`
	}
)

func (b bdcBackend) ref(branch string) dcBranchRef {
	return dcBranchRef{ID: "refs/heads/" + branch, Repository: dcRepoRef{Slug: b.slug, Project: dcProjectRef{Key: b.key}}}
}

// createPR opens a pull request for head against base (#464).
func (b bdcBackend) createPR(ctx context.Context, _, head, base, title string) (int, error) {
	pr, err := sendJSON[struct {
		ID int `json:"id"`
	}](ctx, b.f, "POST", b.repo()+"/pull-requests", dcOpen{Title: title, FromRef: b.ref(head), ToRef: b.ref(base)})
	if err != nil {
		return 0, err
	}
	return opened(pr.ID, b.remote.Host, "pull request")
}

// mergePR merges now, at the version just read - Data Center refuses a merge
// that does not name the pull request's current version - if its head is
// still the one that was green (#599).
func (b bdcBackend) mergePR(ctx context.Context, _ string, number int, head string) (bool, error) {
	path := b.repo() + "/pull-requests/" + strconv.Itoa(number)
	pr, err := getJSON[struct {
		Version int   `json:"version"`
		FromRef dcRef `json:"fromRef"`
	}](ctx, b.f, path)
	if err != nil {
		return false, err
	}
	if err := stillAt(number, pr.FromRef.LatestCommit, head); err != nil {
		return false, err
	}
	got, err := sendJSON[struct {
		State string `json:"state"`
	}](ctx, b.f, "POST", path+"/merge?version="+strconv.Itoa(pr.Version), struct{}{})
	return got.State == "MERGED", err
}

// branchProtected is whether any branch permission covers branch, failing
// closed on a branching-model matcher and on a page left unread.
func (b bdcBackend) branchProtected(ctx context.Context, _, branch string) (bool, error) {
	page, err := getJSON[dcRestrictions](ctx, b.f, "branch-permissions/2.0/projects/"+b.key+"/repos/"+b.slug+"/restrictions?limit=100")
	if err != nil {
		return true, err
	}
	return page.cover(branch), nil
}

func (p dcRestrictions) cover(branch string) bool {
	for _, r := range p.Values {
		m := r.Matcher
		switch m.Type.ID {
		case "BRANCH":
			if m.ID == "refs/heads/"+branch || m.ID == branch {
				return true
			}
		case "PATTERN":
			if globMatches(m.ID, branch) || globMatches(m.ID, "refs/heads/"+branch) {
				return true
			}
		default: // a branching-model matcher only the model can resolve
			return true
		}
	}
	return !p.IsLastPage
}
