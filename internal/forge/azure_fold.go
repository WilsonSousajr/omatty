package forge

import (
	"html"
	"regexp"
	"strings"
	"time"
)

// Azure DevOps's REST answers, as it writes them (#456).
type (
	azList[T any] struct {
		Value []T `json:"value"`
	}
	azIdentity struct {
		DisplayName string `json:"displayName"`
	}
	azPR struct {
		PullRequestID         int        `json:"pullRequestId"`
		Title                 string     `json:"title"`
		Description           string     `json:"description"`
		Status                string     `json:"status"`
		IsDraft               bool       `json:"isDraft"`
		MergeStatus           string     `json:"mergeStatus"`
		SourceRefName         string     `json:"sourceRefName"`
		CreationDate          time.Time  `json:"creationDate"`
		ClosedDate            time.Time  `json:"closedDate"`
		CreatedBy             azIdentity `json:"createdBy"`
		LastMergeSourceCommit struct {
			CommitID string `json:"commitId"`
		} `json:"lastMergeSourceCommit"`
		Repository struct {
			Project struct {
				ID string `json:"id"`
			} `json:"project"`
		} `json:"repository"`
		ForkSource *struct{} `json:"forkSource"`
	}
	azEvaluation struct {
		Status        string `json:"status"`
		Configuration struct {
			IsEnabled bool `json:"isEnabled"`
			Type      struct {
				DisplayName string `json:"displayName"`
			} `json:"type"`
			Settings struct {
				DisplayName string `json:"displayName"`
			} `json:"settings"`
		} `json:"configuration"`
	}
	azThread struct {
		Comments []struct {
			Content       string     `json:"content"`
			CommentType   string     `json:"commentType"`
			IsDeleted     bool       `json:"isDeleted"`
			Author        azIdentity `json:"author"`
			PublishedDate time.Time  `json:"publishedDate"`
		} `json:"comments"`
	}
	azWIQL struct {
		WorkItems []struct {
			ID int `json:"id"`
		} `json:"workItems"`
	}
	azWorkItem struct {
		ID     int `json:"id"`
		Fields struct {
			Title       string     `json:"System.Title"`
			Tags        string     `json:"System.Tags"`
			Description string     `json:"System.Description"`
			AssignedTo  azIdentity `json:"System.AssignedTo"`
			CreatedBy   azIdentity `json:"System.CreatedBy"`
			CreatedDate time.Time  `json:"System.CreatedDate"`
			ChangedDate time.Time  `json:"System.ChangedDate"`
		} `json:"fields"`
	}
	azComments struct {
		Comments []struct {
			Text        string     `json:"text"`
			CreatedBy   azIdentity `json:"createdBy"`
			CreatedDate time.Time  `json:"createdDate"`
		} `json:"comments"`
	}
)

// foldAzPRs is Azure's pull requests as omatty's. Azure's list carries no
// update time, so a closed one's close and an open one's creation stand in.
func foldAzPRs(in []azPR) []PR {
	out := make([]PR, len(in))
	for i, p := range in {
		out[i] = PR{
			Number: p.PullRequestID, Title: cleanLine(p.Title),
			Branch: cleanLine(strings.TrimPrefix(p.SourceRefName, "refs/heads/")),
			State:  azState(p.Status), Head: p.LastMergeSourceCommit.CommitID, Draft: p.IsDraft,
			Conflict: p.MergeStatus == "conflicts", Fork: p.ForkSource != nil,
			Updated: p.CreationDate,
		}
		if !p.ClosedDate.IsZero() {
			out[i].Updated = p.ClosedDate
		}
		if out[i].State == Merged {
			out[i].MergedAt = p.ClosedDate
		}
	}
	return out
}

func azState(s string) PRState {
	switch s {
	case "completed":
		return Merged
	case "abandoned":
		return Closed
	}
	return Open
}

// buildPolicies is the evaluations that are CI: enabled build validation, not
// a reviewer count or a linked work item.
func buildPolicies(in []azEvaluation) []azEvaluation {
	var out []azEvaluation
	for _, e := range in {
		if e.Configuration.IsEnabled && e.Configuration.Type.DisplayName == "Build" {
			out = append(out, e)
		}
	}
	return out
}

// azureCI is a build policy's status as the card's CI mark; an unknown one is
// running, never passing.
func azureCI(status string) CIState {
	switch status {
	case "approved":
		return CIPassing
	case "rejected", "broken":
		return CIFailing
	}
	return CIRunning
}

func (a azBackend) foldItems(in []azWorkItem) []Issue {
	out := make([]Issue, len(in))
	for i, w := range in {
		out[i] = issueOf(w.ID, w.Fields.Title, tags(w.Fields.Tags), w.Fields.AssignedTo.DisplayName,
			w.Fields.CreatedBy.DisplayName, w.Fields.ChangedDate, a.itemURL(w.ID))
	}
	return out
}

// tags is Azure's "bug; wiki" as labels.
func tags(s string) []string {
	var out []string
	for _, t := range strings.Split(s, ";") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func (w azWorkItem) flat(url string, comments []struct {
	Text        string     `json:"text"`
	CreatedBy   azIdentity `json:"createdBy"`
	CreatedDate time.Time  `json:"createdDate"`
}) ghDetail {
	d := ghDetail{Number: w.ID, Title: w.Fields.Title, Body: htmlText(w.Fields.Description), URL: url,
		CreatedAt: w.Fields.CreatedDate, Author: ghUser{Login: w.Fields.CreatedBy.DisplayName}}
	for _, c := range comments {
		d.Comments = append(d.Comments, ghComment{Author: ghUser{Login: c.CreatedBy.DisplayName}, Body: htmlText(c.Text), CreatedAt: c.CreatedDate})
	}
	return d
}

// flat is the pull request as gh's detail type: its people's comments, not the
// system's or the deleted, and each build policy a check.
func (p azPR) flat(url string, threads []azThread, builds []azEvaluation) ghDetail {
	d := ghDetail{Number: p.PullRequestID, Title: p.Title, Body: p.Description, URL: url,
		CreatedAt: p.CreationDate, Author: ghUser{Login: p.CreatedBy.DisplayName}}
	for _, t := range threads {
		for _, c := range t.Comments {
			if c.CommentType == "text" && !c.IsDeleted {
				d.Comments = append(d.Comments, ghComment{Author: ghUser{Login: c.Author.DisplayName}, Body: c.Content, CreatedAt: c.PublishedDate})
			}
		}
	}
	for _, b := range builds {
		d.Checks = append(d.Checks, statusCheck(b.Configuration.Settings.DisplayName, azureCI(b.Status), time.Time{}, time.Time{}))
	}
	return d
}

var (
	htmlBreak = regexp.MustCompile(`(?i)<br\s*/?>|</(div|p|li|h[1-6])>`)
	htmlTag   = regexp.MustCompile(`<[^>]*>`)
)

// htmlText is a work item's HTML as the text a terminal can show: a break or a
// block's end a line, every other tag gone, entities read.
func htmlText(s string) string {
	s = htmlBreak.ReplaceAllString(s, "\n")
	s = html.UnescapeString(htmlTag.ReplaceAllString(s, ""))
	return strings.TrimSpace(s)
}
