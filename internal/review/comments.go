package review

import "time"

// Comment is one queued review note. Quote is the line's text when the note
// was written, so it can still be shown and sent after the line moves or
// disappears (#22). Sent is when it went to the session; zero means pending.
type Comment struct {
	Anchor Anchor
	Quote  string
	Note   string
	Sent   time.Time
}

// Comments is one session's in-memory queue. Submitting marks what it sent
// rather than dropping it, so the next turn can be read against what was
// asked (#335). Quitting omatty drops it.
//
//	cs := review.NewComments()
//	cs.Add(review.Comment{Anchor: a, Quote: "b := 3", Note: "why 3?"})
type Comments struct{ queued []Comment }

// NewComments returns an empty queue.
func NewComments() *Comments { return &Comments{} }

// Add appends cm to the queue.
func (c *Comments) Add(cm Comment) { c.queued = append(c.queued, cm) }

// All returns the queue in order, as a copy the caller may keep.
func (c *Comments) All() []Comment { return append([]Comment(nil), c.queued...) }

// Len is the number of pending comments.
func (c *Comments) Len() int { return len(c.queued) }

// Remove drops the i-th comment; false when i is out of range.
func (c *Comments) Remove(i int) bool {
	if i < 0 || i >= len(c.queued) {
		return false
	}
	c.queued = append(c.queued[:i], c.queued[i+1:]...)
	return true
}

// Pending returns the comments not yet sent, in order - what a submit sends.
func (c *Comments) Pending() []Comment {
	var out []Comment
	for _, cm := range c.queued {
		if cm.Sent.IsZero() {
			out = append(out, cm)
		}
	}
	return out
}

// PendingLen is the number of comments not yet sent.
func (c *Comments) PendingLen() int { return len(c.Pending()) }

// MarkSent records every pending comment as sent at at. A comment already
// sent keeps its own time: it says when that note went, not the latest batch.
func (c *Comments) MarkSent(at time.Time) {
	for i := range c.queued {
		if c.queued[i].Sent.IsZero() {
			c.queued[i].Sent = at
		}
	}
}

// PruneSent drops every sent comment that no longer resolves to a line of d.
// Its line or file is gone, so whatever it asked for was dealt with one way or
// another; a pending comment in the same place stays and shows as moved.
func (c *Comments) PruneSent(d Diff) {
	placed := Place(d, c.queued)
	kept := c.queued[:0]
	for i, cm := range c.queued {
		if _, onALine := placed.Where[i]; onALine || cm.Sent.IsZero() {
			kept = append(kept, cm)
		}
	}
	c.queued = kept
}
