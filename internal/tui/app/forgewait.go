package app

import (
	"errors"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/forge"
)

// The backoff for a rate limit that named no time - Bitbucket Cloud's 429 may
// carry no header (#594). Five minutes is five polls: a minute's wait would be
// no wait at all against a one-minute tick. An hour is the window Atlassian's
// limit is counted over, so no wait need be longer.
const (
	waitFirst = 5 * time.Minute
	waitMost  = time.Hour
)

// hostWait is a rate-limited host: when its wait ends, and the backoff the
// next limit with no time doubles from.
type hostWait struct {
	until   time.Time
	backoff time.Duration
}

// waitHost starts the wait on project's host when err is a rate limit, and
// reports whether it was. The quota is the account's on that host, so every
// project there waits, the ones that never saw the 429 included (#594).
func (m *Model) waitHost(project string, err error) bool {
	var limited *forge.RateLimitError
	if !errors.As(err, &limited) {
		return false
	}
	host := m.host(project)
	w := m.forgeWait[host]
	if limited.Until.IsZero() {
		w.backoff = min(max(w.backoff*2, waitFirst), waitMost)
		w.until = m.clock().Add(w.backoff)
	} else {
		w.until = limited.Until
	}
	m.forgeWait[host] = w
	return true
}

// hostWaiting is when the wait on project's host ends, and whether it has not
// yet.
func (m *Model) hostWaiting(project string) (time.Time, bool) {
	w, ok := m.forgeWait[m.host(project)]
	return w.until, ok && m.clock().Before(w.until)
}

// hostAnswered lifts the wait on project's host once it is over, and resets
// its backoff. An answer that lands while the wait still runs was asked for
// before the limit, so it says nothing about whether the limit has passed.
func (m *Model) hostAnswered(project string) {
	if _, waiting := m.hostWaiting(project); !waiting {
		delete(m.forgeWait, m.host(project))
	}
}

// unknownMark is the tracker title's mark for a stale list: "?", and until
// when, while its host waits out a rate limit.
func (m *Model) unknownMark(project string) string {
	if until, waiting := m.hostWaiting(project); waiting {
		return "? until " + until.Format("15:04")
	}
	return "?"
}

// waitNote is the tracker's line for a project whose host is waiting.
func (m *Model) waitNote(project string) (string, bool) {
	until, waiting := m.hostWaiting(project)
	if !waiting {
		return "", false
	}
	return "rate-limited by " + m.host(project) + " until " + until.Format("15:04") + "; omatty asks again then.", true
}
