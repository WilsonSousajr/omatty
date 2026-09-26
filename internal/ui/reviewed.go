// Per-file "I have read this", and "it changed since I did" (#337).

package ui

import "github.com/WilsonSousajr/omatty/internal/review"

// The review gutter's three states. One cell, left of the change letter, so
// every row a session did not touch still reads exactly as it did: the change
// letter and the name keep their places.
const (
	reviewedMark     = "✓" // read, and unchanged since
	changedSinceMark = "~" // read, and the session has changed it again
	unreviewedMark   = " "
)

// toggleReviewed marks the file under the tree cursor as read, or takes the
// mark back. The digest stored is the file's diff content, so the mark is
// about a change rather than about a path (#337).
func (m *Model) toggleReviewed() {
	rows := m.treeRows()
	if m.review.TreeCursor >= len(rows) {
		return
	}
	n := rows[m.review.TreeCursor]
	// A directory is not a review, and enter already means something on it.
	if n.IsDir {
		return
	}
	digest, changed := m.diffDigest(n.Path)
	if !changed {
		// Saying nothing would read as a broken key. "Changed since reviewed"
		// has no meaning for a file the session never touched, which is the
		// honest reason and the one the footer gives.
		m.notice = n.Path + " is not changed in this session; there is nothing to review"
		return
	}
	m.flipReviewed(n.Path, digest)
	m.contentChanged()
}

// flipReviewed records path as read at digest, or forgets it when it was
// already marked - which is how a mark made by mistake is taken back.
func (m *Model) flipReviewed(path, digest string) {
	marks := m.reviewedFor(m.review.SessionID)
	if _, marked := marks[path]; marked {
		delete(marks, path)
		return
	}
	marks[path] = digest
}

// reviewMark is the gutter cell for path: read and unchanged, read and changed
// again, or nothing.
//
// A file that has left the diff altogether reads as unmarked rather than as
// reviewed: there is no change to have read. The stored digest stays, so if the
// same change comes back it is recognised.
func (m *Model) reviewMark(path string, isDir bool) string {
	if isDir {
		return unreviewedMark
	}
	was, marked := m.reviewedFor(m.review.SessionID)[path]
	if !marked {
		return unreviewedMark
	}
	now, changed := m.diffDigest(path)
	switch {
	case !changed:
		return unreviewedMark
	case now == was:
		return reviewedMark
	}
	return changedSinceMark
}

// diffDigest is the current digest of path's change, and whether the shown
// diff changes it at all.
func (m *Model) diffDigest(path string) (string, bool) {
	for _, f := range m.shownDiff().Files {
		if f.Path == path {
			return review.FileDigest(f), true
		}
	}
	return "", false
}

// reviewedFor is the session's marks, allocated on first use so the caller
// never writes into a nil map.
func (m *Model) reviewedFor(id string) map[string]string {
	if m.reviewed[id] == nil {
		m.reviewed[id] = map[string]string{}
	}
	return m.reviewed[id]
}
