package forge

import "time"

// Gitea's REST answers, as it writes them (#458).
type (
	gtUser struct {
		Login string `json:"login"`
	}
	gtRef struct {
		Ref  string `json:"ref"`
		SHA  string `json:"sha"`
		Repo *struct {
			ID int `json:"id"`
		} `json:"repo"`
	}
	gtPR struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		Head      gtRef     `json:"head"`
		Base      gtRef     `json:"base"`
		State     string    `json:"state"`
		Merged    bool      `json:"merged"`
		MergedAt  time.Time `json:"merged_at"`
		Draft     bool      `json:"draft"`
		Mergeable bool      `json:"mergeable"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	gtIssue struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		Labels    []ghLabel `json:"labels"`
		Assignees []gtUser  `json:"assignees"`
		User      gtUser    `json:"user"`
		UpdatedAt time.Time `json:"updated_at"`
		HTMLURL   string    `json:"html_url"`
	}
	gtItem struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		Body      string    `json:"body"`
		User      gtUser    `json:"user"`
		CreatedAt time.Time `json:"created_at"`
		HTMLURL   string    `json:"html_url"`
		Head      gtRef     `json:"head"`
	}
	gtComment struct {
		Body      string    `json:"body"`
		User      gtUser    `json:"user"`
		CreatedAt time.Time `json:"created_at"`
	}
	gtStatus struct {
		State      string          `json:"state"`
		TotalCount int             `json:"total_count"`
		Statuses   []gtCommitCheck `json:"statuses"`
	}
	gtCommitCheck struct {
		Context   string    `json:"context"`
		Status    string    `json:"status"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
)

func foldGTPRs(in []gtPR) []PR {
	out := make([]PR, len(in))
	for i, p := range in {
		out[i] = PR{
			Number: p.Number, Title: cleanLine(p.Title), Branch: cleanLine(p.Head.Ref), Base: cleanLine(p.Base.Ref),
			State: gtState(p), Head: p.Head.SHA, Draft: p.Draft,
			Conflict: gtConflict(p), Fork: gtFork(p),
			Updated: p.UpdatedAt, MergedAt: p.MergedAt,
		}
	}
	return out
}

func gtState(p gtPR) PRState {
	switch {
	case p.Merged:
		return Merged
	case p.State == "closed":
		return Closed
	}
	return Open
}

// gtConflict is an open pull request that cannot merge. Gitea reports every
// draft as not mergeable, so a draft is not a conflict (#458's review).
func gtConflict(p gtPR) bool { return p.State == "open" && !p.Mergeable && !p.Draft }

// gtFork is whether the head lives in another repository; a head whose
// repository was deleted is a fork's that is gone.
func gtFork(p gtPR) bool {
	return p.Head.Repo == nil || p.Base.Repo == nil || p.Head.Repo.ID != p.Base.Repo.ID
}

// giteaCI is a commit status as the card's CI mark. A warning is not a pass:
// Gitea's own Combine() makes it a failure, and Forgejo ranks it worse than
// pending, so a combined "warning" can hide a check still running (#458's
// review). A status omatty does not know is running, never passing.
func giteaCI(status string) CIState {
	switch status {
	case "success", "skipped":
		return CIPassing
	case "failure", "error", "warning":
		return CIFailing
	}
	return CIRunning
}

func foldGTIssues(in []gtIssue) []Issue {
	out := make([]Issue, len(in))
	for i, is := range in {
		assignee := ""
		if len(is.Assignees) > 0 {
			assignee = is.Assignees[0].Login
		}
		out[i] = issueOf(is.Number, is.Title, labelNames(is.Labels), assignee, is.User.Login, is.UpdatedAt, is.HTMLURL)
	}
	return out
}

// flat is the item as gh's detail type, so gh's fold bounds and cleans it.
func (it gtItem) flat(comments []gtComment, checks []gtCommitCheck) ghDetail {
	d := ghDetail{
		Number: it.Number, Title: it.Title, Body: it.Body, URL: it.HTMLURL,
		CreatedAt: it.CreatedAt, Author: ghUser{Login: it.User.Login},
	}
	for _, c := range comments {
		d.Comments = append(d.Comments, ghComment{Author: ghUser{Login: c.User.Login}, Body: c.Body, CreatedAt: c.CreatedAt})
	}
	for _, c := range checks {
		d.Checks = append(d.Checks, statusCheck(c.Context, giteaCI(c.Status), c.CreatedAt, c.UpdatedAt))
	}
	return d
}
