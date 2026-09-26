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
		fmt.Fprintf(&b, "\n%d. %s\n   > %s\n%s   %s\n",
			i+1, locate(d, p, i, c), c.Quote, about(c), c.Note)
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

// about names the part of the line a note is about, or nothing for a note
// about all of it (#339).
//
// It says when the fragment has gone. A line can survive an edit that removes
// the words the note was written about, and quoting them as though they were
// still there would be telling claude to look at something that is not on the
// line - so the note travels with the truth attached. The operator wrote it
// about something, and dropping it silently is worse than saying it moved,
// which is the argument locate() already makes for a whole comment.
func about(c Comment) string {
	if c.Fragment == "" {
		return ""
	}
	if !strings.Contains(c.Quote, c.Fragment) {
		return fmt.Sprintf("   about: %q (no longer in this line)\n", c.Fragment)
	}
	return fmt.Sprintf("   about: %q\n", c.Fragment)
}

// lineNumber is the new-file number, or the old-file number of a removed
// line, which has no new one.
func lineNumber(l Line) int {
	if l.NewNo > 0 {
		return l.NewNo
	}
	return l.OldNo
}
