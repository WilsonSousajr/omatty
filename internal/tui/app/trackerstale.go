// The tracker's line over a list a refresh failed to renew (#776).
//
// A failed poll keeps the last list rather than reading as empty, and the
// title has said so with a ? since #310 - but a ? says "stale", not why: a
// network blip and a CLI that needs logging in looked the same, and the
// error was only in the log. The list now carries one muted notice above it:
// what failed, how old the list on screen is, and the key that asks again.

package app

import "time"

// staleLines is the notice, wrapped to the column; nil when the last refresh
// answered, or when there is no list for it to stand over - trackerNote says
// what is wrong then.
func (m *Model) staleLines() []string {
	read, err := m.refreshFailure(m.review.Tracker.Project)
	if err == nil || len(m.trackerRows()) == 0 {
		return nil
	}
	text := "could not refresh (" + causeOf(err) + ")"
	if age := AgeString(m.clock(), read); age != "" {
		text += " · showing " + age + " old"
	}
	return noticeLines([]string{text + " · r retries"}, false, m.columnWidth())
}

// refreshFailure is a failed list's error and when that list last answered:
// the issues' first, since they lead the tracker.
func (m *Model) refreshFailure(project string) (time.Time, error) {
	if err := m.issueFailed[project]; err != nil {
		return m.issueRead[project], err
	}
	return m.prRead[project], m.prFailed[project]
}

// listLead is how many rows above a face's list are not the list: the
// tracker's notice. The cursor's window and a click's row both start below it.
func (m *Model) listLead() int {
	if m.review.View != ViewTracker {
		return 0
	}
	return len(m.staleLines())
}

// trackerRowsShown is how many of the tracker's rows the window holds.
func (m *Model) trackerRowsShown() int { return m.reviewRows() - m.listLead() }
