package review

// Comment is one queued review note. Quote is the line's text when the note
// was written, so it can still be shown and sent after the line moves or
// disappears (#22).
type Comment struct {
	Anchor Anchor
	Quote  string
	Note   string
}

// Comments is one session's in-memory queue, drained by submit. Quitting
// omatty drops it; persistence is M6.
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

// Clear empties the queue after a submit.
func (c *Comments) Clear() { c.queued = nil }
