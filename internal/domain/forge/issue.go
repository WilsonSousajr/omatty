package forge

import "time"

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

// ListWindow is how many open issues, and how many open pull requests, one
// read returns: every backend asks for its first hundred, newest first. A
// list that comes back this long may have been cut, and the window says so
// rather than reading "100" as all of them (#658).
//
//	if len(issues) >= forge.ListWindow { title += "+" }
const ListWindow = 100
