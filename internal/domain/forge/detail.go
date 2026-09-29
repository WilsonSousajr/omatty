package forge

import "time"

// DetailMax bounds an item's text. Someone else's issue can be any size, and a
// pane must not hold megabytes of it - the preview's argument (#24), at the same
// 64 KiB the preview draws plain above.
const DetailMax = 64 << 10

// Comment is one comment on an item.
type Comment struct {
	Author string
	Body   string
	At     time.Time
}

// Detail is one issue or pull request in full.
type Detail struct {
	Number   int
	Title    string
	Author   string
	Body     string
	Comments []Comment
	URL      string
	Created  time.Time
	// Truncated says the body or the comments were cut at DetailMax. A short
	// item must never read as the whole one.
	Truncated bool
	// Checks is a pull request's CI, one per check, in gh's order (#433).
	Checks []Check
}

// Check is one CI check on a pull request: its name, how it stands, and how
// long it ran - zero for one still running or never started.
type Check struct {
	Name  string
	State CIState
	Took  time.Duration
}
