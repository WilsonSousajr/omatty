package review

import (
	"fmt"
	"strings"
)

// Compose writes the one message [S] sends: every queued comment as
// file:line, the quoted line and the note, numbered so Claude can answer them
// one by one (#23). Line numbers come from the current diff, so they are the
// ones Claude sees now, not the ones that held when the note was written; an
// orphan names the file alone and says the line moved.
//
//	body := review.Compose(d, cs.All())
func Compose(d Diff, comments []Comment) string {
	p := Place(d, comments)
	var b strings.Builder
	fmt.Fprintf(&b, "Review comments (%d):\n", len(comments))
	for i, c := range comments {
		fmt.Fprintf(&b, "\n%d. %s\n   > %s\n   %s\n", i+1, locate(d, p, i, c), c.Quote, c.Note)
	}
	return strings.TrimRight(b.String(), "\n")
}

// locate renders where comment i sits now. A comment whose line or file is
// gone still travels: the operator wrote it about something, and dropping it
// silently is worse than citing a path.
func locate(d Diff, p Placed, i int, c Comment) string {
	pos, ok := p.Where[i]
	if !ok {
		return c.Anchor.File + " (line moved or removed)"
	}
	return fmt.Sprintf("%s:%d", c.Anchor.File, lineNumber(d.LineAt(pos)))
}

// lineNumber is the new-file number, or the old-file number of a removed
// line, which has no new one.
func lineNumber(l Line) int {
	if l.NewNo > 0 {
		return l.NewNo
	}
	return l.OldNo
}
