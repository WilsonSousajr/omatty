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
