// The other half of what a forge holds (#393): a project's open issues, read
// the same way its pull requests are - one call for the whole project, folded
// into typed values, never one call per item (#358).

package forge

import (
	"encoding/json"
	"fmt"
	"time"
)

// issueFields is everything FoldIssues reads. A tracker row is a number, one
// label, a title and an age; the url is what the browse key and the paste
// reference need.
const issueFields = "number,title,labels,assignees,author,updatedAt,url"

// issueWindow is how many open issues are read. The same hundred the open pull
// requests get, and for the same reason: an open item matters however old it
// is, so the window is the whole open set in every repository this is for.
const issueWindow = "100"

// Issue is one open issue as the tracker's list needs it.
type Issue struct {
	Number   int
	Title    string
	Labels   []string // names only, in the order gh lists them
	Assignee string   // the first assignee's login, or "" when nobody is on it
	Author   string
	Updated  time.Time
	URL      string
}

// ghIssue is one element of `gh issue list --json` with the fields ListIssues
// asks for.
type ghIssue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Labels    []ghLabel `json:"labels"`
	Assignees []ghUser  `json:"assignees"`
	Author    ghUser    `json:"author"`
	UpdatedAt time.Time `json:"updatedAt"`
	URL       string    `json:"url"`
}

// ghLabel and ghUser are the two nested objects gh answers with. Named types
// rather than map[string]any, which no package boundary here carries.
type ghLabel struct {
	Name string `json:"name"`
}

type ghUser struct {
	Login string `json:"login"`
}

// FoldIssues turns `gh issue list --json` output into Issues.
//
//	issues, err := forge.FoldIssues(out)
func FoldIssues(raw []byte) ([]Issue, error) {
	var in []ghIssue
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("forge: reading gh's issue list: %w", err)
	}
	out := make([]Issue, len(in))
	for i, is := range in {
		out[i] = Issue{
			Number:   is.Number,
			Title:    cleanLine(is.Title), // #483, as every field an author controls
			Labels:   labelNames(is.Labels),
			Assignee: cleanLine(firstLogin(is.Assignees)),
			Author:   cleanLine(is.Author.Login),
			Updated:  is.UpdatedAt,
			URL:      cleanLine(is.URL),
		}
	}
	return out, nil
}

func labelNames(in []ghLabel) []string {
	if len(in) == 0 {
		return nil
	}
	names := make([]string, len(in))
	for i, l := range in {
		names[i] = cleanLine(l.Name) // #483
	}
	return names
}

// firstLogin is the one assignee a row has room for. Nobody assigned is the
// common case in a one-person repository, and it is an empty name, not a gap
// in the answer.
func firstLogin(in []ghUser) string {
	if len(in) == 0 {
		return ""
	}
	return in[0].Login
}
