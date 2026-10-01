package forge

import (
	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
	"time"
)

// The GitLab REST API's answers, as it writes them (#454).
type (
	glMR struct {
		IID                 int       `json:"iid"`
		Title               string    `json:"title"`
		SourceBranch        string    `json:"source_branch"`
		TargetBranch        string    `json:"target_branch"`
		SHA                 string    `json:"sha"`
		State               string    `json:"state"`
		Draft               bool      `json:"draft"`
		HasConflicts        bool      `json:"has_conflicts"`
		DetailedMergeStatus string    `json:"detailed_merge_status"`
		SourceProjectID     int       `json:"source_project_id"`
		TargetProjectID     int       `json:"target_project_id"`
		UpdatedAt           time.Time `json:"updated_at"`
		MergedAt            time.Time `json:"merged_at"`
	}
	glUser struct {
		Username string `json:"username"`
	}
	glIssue struct {
		IID       int       `json:"iid"`
		Title     string    `json:"title"`
		Labels    []string  `json:"labels"`
		Assignees []glUser  `json:"assignees"`
		Author    glUser    `json:"author"`
		UpdatedAt time.Time `json:"updated_at"`
		WebURL    string    `json:"web_url"`
	}
	glItem struct {
		IID          int         `json:"iid"`
		Title        string      `json:"title"`
		Description  string      `json:"description"`
		Author       glUser      `json:"author"`
		CreatedAt    time.Time   `json:"created_at"`
		WebURL       string      `json:"web_url"`
		HeadPipeline *glPipeline `json:"head_pipeline"`
	}
	glNote struct {
		Body      string    `json:"body"`
		Author    glUser    `json:"author"`
		CreatedAt time.Time `json:"created_at"`
		System    bool      `json:"system"`
	}
	glPipeline struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
	}
	glJob struct {
		Name       string    `json:"name"`
		Status     string    `json:"status"`
		StartedAt  time.Time `json:"started_at"`
		FinishedAt time.Time `json:"finished_at"`
	}
)

// foldMRs is GitLab's merge requests as omatty's pull requests. CI is filled
// after, from each one's own pipeline.
func foldMRs(in []glMR) []dforge.PR {
	out := make([]dforge.PR, len(in))
	for i, m := range in {
		out[i] = dforge.PR{
			Number: m.IID, Title: cleanLine(m.Title), Branch: cleanLine(m.SourceBranch), Base: cleanLine(m.TargetBranch),
			State: glState(m.State), Head: m.SHA, Draft: m.Draft,
			Conflict: m.HasConflicts || m.DetailedMergeStatus == "conflict" || m.DetailedMergeStatus == "need_rebase",
			Fork:     m.SourceProjectID != m.TargetProjectID,
			Updated:  m.UpdatedAt, MergedAt: m.MergedAt, Review: glReview(m.DetailedMergeStatus),
		}
	}
	return out
}

// glState is GitLab's state as a PRState; locked is a closed discussion.
func glState(s string) dforge.PRState {
	switch s {
	case "merged":
		return dforge.Merged
	case "closed", "locked":
		return dforge.Closed
	}
	return dforge.Open
}

// glReview reads what GitLab's merge status says about review: it names
// changes requested and approval still owed, and nothing else.
func glReview(status string) dforge.Review {
	switch status {
	case "requested_changes":
		return dforge.ReviewChanges
	case "not_approved":
		return dforge.ReviewRequired
	}
	return dforge.ReviewNone
}

// gitlabCI is a pipeline's or a job's status as the card's CI mark. A status
// omatty does not know is running, never passing: unknown is not shown as a
// green verdict.
func gitlabCI(status string) dforge.CIState {
	switch status {
	case "success", "skipped":
		return dforge.CIPassing
	case "failed", "canceled":
		return dforge.CIFailing
	}
	return dforge.CIRunning
}

func foldGLIssues(in []glIssue) []dforge.Issue {
	out := make([]dforge.Issue, len(in))
	for i, is := range in {
		assignee := ""
		if len(is.Assignees) > 0 {
			assignee = is.Assignees[0].Username
		}
		out[i] = issueOf(is.IID, is.Title, is.Labels, assignee, is.Author.Username, is.UpdatedAt, is.WebURL)
	}
	return out
}

// issueOf is one issue as omatty's own type, every field an author controls
// cleaned (#483). Each forge's fold calls it with its own field names.
func issueOf(number int, title string, labels []string, assignee, author string, updated time.Time, url string) dforge.Issue {
	return dforge.Issue{
		Number: number, Title: cleanLine(title), Labels: cleanLabels(labels),
		Assignee: cleanLine(assignee), Author: cleanLine(author), Updated: updated, URL: cleanLine(url),
	}
}

func cleanLabels(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	for i, l := range in {
		out[i] = cleanLine(l)
	}
	return out
}

// flat is the item as gh's detail type, so gh's fold bounds and cleans it:
// comments without system notes, and each job a check.
func (it glItem) flat(notes []glNote, jobs []glJob) ghDetail {
	d := ghDetail{
		Number: it.IID, Title: it.Title, Body: it.Description, URL: it.WebURL,
		CreatedAt: it.CreatedAt, Author: ghUser{Login: it.Author.Username},
	}
	for _, n := range notes {
		if !n.System {
			d.Comments = append(d.Comments, ghComment{Author: ghUser{Login: n.Author.Username}, Body: n.Body, CreatedAt: n.CreatedAt})
		}
	}
	for _, j := range jobs {
		d.Checks = append(d.Checks, statusCheck(j.Name, gitlabCI(j.Status), j.StartedAt, j.FinishedAt))
	}
	return d
}

// statusCheck is one check in gh's StatusContext shape, whose state gh's own
// rollup reads back as the same verdict.
func statusCheck(name string, state dforge.CIState, started, finished time.Time) check {
	states := map[dforge.CIState]string{dforge.CIPassing: "SUCCESS", dforge.CIFailing: "FAILURE", dforge.CIRunning: "PENDING", dforge.CINone: "PENDING"}
	return check{Typename: "StatusContext", Context: name, State: states[state], StartedAt: started, CompletedAt: finished}
}
