package forge

import "time"

// Bitbucket Cloud's REST answers, as it writes them (#460).
type (
	bbPage[T any] struct {
		Values []T    `json:"values"`
		Next   string `json:"next"` // the next page's URL; omatty builds its own, and reads only whether there is one
	}
	bbUser struct {
		Nickname    string `json:"nickname"`
		DisplayName string `json:"display_name"`
	}
	bbEnd struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
		Commit struct {
			Hash string `json:"hash"`
		} `json:"commit"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	bbPR struct {
		ID          int       `json:"id"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		State       string    `json:"state"`
		Draft       bool      `json:"draft"`
		Author      bbUser    `json:"author"`
		Source      bbEnd     `json:"source"`
		Destination bbEnd     `json:"destination"`
		CreatedOn   time.Time `json:"created_on"`
		UpdatedOn   time.Time `json:"updated_on"`
		Links       struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
		} `json:"links"`
	}
	bbComment struct {
		Content struct {
			Raw string `json:"raw"`
		} `json:"content"`
		User      bbUser    `json:"user"`
		CreatedOn time.Time `json:"created_on"`
		Deleted   bool      `json:"deleted"`
	}
	bbStatus struct {
		State     string    `json:"state"`
		Name      string    `json:"name"`
		CreatedOn time.Time `json:"created_on"`
		UpdatedOn time.Time `json:"updated_on"`
	}
)

// foldBBPRs is Bitbucket's pull requests as omatty's. Bitbucket keeps no
// merge time, so a merged one's last update stands in for it (#332's lead
// time); its list says nothing of conflicts, so none is claimed.
func foldBBPRs(in []bbPR) []PR {
	out := make([]PR, len(in))
	for i, p := range in {
		out[i] = PR{
			Number: p.ID, Title: cleanLine(p.Title), Branch: cleanLine(p.Source.Branch.Name),
			State: bbState(p.State), Head: p.Source.Commit.Hash, Draft: p.Draft,
			Fork:    p.Source.Repository.FullName != p.Destination.Repository.FullName,
			Updated: p.UpdatedOn,
		}
		if out[i].State == Merged {
			out[i].MergedAt = p.UpdatedOn
		}
	}
	return out
}

func bbState(s string) PRState {
	switch s {
	case "MERGED":
		return Merged
	case "DECLINED", "SUPERSEDED":
		return Closed
	}
	return Open
}

// bitbucketCI is a commit status as the card's CI mark; an unknown one is
// running, never passing.
func bitbucketCI(state string) CIState {
	switch state {
	case "SUCCESSFUL":
		return CIPassing
	// STOPPED is a run someone halted, and CANCELLED Data Center's STOPPED:
	// neither passed.
	case "FAILED", "STOPPED", "CANCELLED":
		return CIFailing
	case "UNKNOWN": // Data Center's no result - never running forever (#461's review)
		return CINone
	}
	return CIRunning
}

// name is who a Bitbucket user is to a reader: the nickname, else the name.
func (u bbUser) name() string {
	if u.Nickname != "" {
		return u.Nickname
	}
	return u.DisplayName
}

// flat is the pull request as gh's detail type, so gh's fold bounds and
// cleans it: deleted comments left out, each status a check.
func (p bbPR) flat(comments []bbComment, statuses []bbStatus) ghDetail {
	d := ghDetail{
		Number: p.ID, Title: p.Title, Body: p.Description, URL: p.Links.HTML.Href,
		CreatedAt: p.CreatedOn, Author: ghUser{Login: p.Author.name()},
	}
	for _, c := range comments {
		if !c.Deleted {
			d.Comments = append(d.Comments, ghComment{Author: ghUser{Login: c.User.name()}, Body: c.Content.Raw, CreatedAt: c.CreatedOn})
		}
	}
	for _, s := range statuses {
		d.Checks = append(d.Checks, statusCheck(s.Name, bitbucketCI(s.State), s.CreatedOn, s.UpdatedOn))
	}
	return d
}
