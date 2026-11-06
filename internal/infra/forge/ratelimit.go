package forge

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// retryAt is when a forge said to ask again: Retry-After in seconds or as an
// HTTP date, else the X-RateLimit-Reset epoch GitHub and GitLab send. Zero
// when it named neither, or named one omatty cannot read - Bitbucket Cloud's
// 429 may carry no header at all - so the caller backs off on its own rather
// than taking an unreadable header as "now" (#594).
func retryAt(h http.Header, now time.Time) time.Time {
	if after := h.Get("Retry-After"); after != "" {
		if secs, err := strconv.Atoi(after); err == nil {
			return now.Add(time.Duration(secs) * time.Second)
		}
		if at, err := http.ParseTime(after); err == nil {
			return at
		}
	}
	if epoch, err := strconv.ParseInt(h.Get("X-Ratelimit-Reset"), 10, 64); err == nil {
		return time.Unix(epoch, 0)
	}
	return time.Time{}
}

// saysRateLimit is whether a forge's words name a rate limit: GitHub's 403
// body, gh's stderr, a CLI's error line.
func saysRateLimit(text string) bool {
	return strings.Contains(strings.ToLower(text), "rate limit")
}
