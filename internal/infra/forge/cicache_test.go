package forge

import (
	"context"
	"errors"
	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
	"strconv"
	"sync"
	"testing"
	"time"
)

// Regression, #454's review: a running change is never kept, so the same thirty
// most recently updated were asked every poll and older ones never. The
// changes least recently asked go first, so every one is reached in turn.
func TestCICache_AsksTheLeastRecentlyAskedFirst_issue454(t *testing.T) {
	c := newCICache(time.Now)
	prs := make([]dforge.PR, 40)
	for i := range prs {
		prs[i] = dforge.PR{Number: i + 1, State: dforge.Open, Head: "h" + strconv.Itoa(i), Updated: time.Unix(int64(1000-i), 0)}
	}
	key := func(pr dforge.PR) string { return pr.Head }
	var mu sync.Mutex
	var asked []int
	ask := func(_ context.Context, pr dforge.PR) (dforge.CIState, error) {
		mu.Lock()
		defer mu.Unlock()
		asked = append(asked, pr.Number)
		return dforge.CIRunning, nil
	}

	_ = c.fill(context.Background(), prs, key, ask)
	first := len(asked)
	asked = nil
	_ = c.fill(context.Background(), prs, key, ask)

	seen := map[int]bool{}
	for _, n := range asked {
		seen[n] = true
	}
	for n := 31; n <= 40; n++ {
		if !seen[n] {
			t.Errorf("poll 2 did not reach #%d, which poll 1 (%d asks) left unasked", n, first)
		}
	}
}

// fakeClock is a time a test moves by hand.
type fakeClock struct{ at time.Time }

func (c *fakeClock) now() time.Time { return c.at }

// oneOpen is a single open change, with its key and an ask that answers state
// and counts.
func oneOpen(state dforge.CIState) ([]dforge.PR, func(dforge.PR) string, func(context.Context, dforge.PR) (dforge.CIState, error), *int) {
	asks := 0
	prs := []dforge.PR{{Number: 1, State: dforge.Open, Head: "h1"}}
	ask := func(context.Context, dforge.PR) (dforge.CIState, error) { asks++; return state, nil }
	return prs, func(pr dforge.PR) string { return pr.Head }, ask, &asks
}

// keptFor is how many asks a change answering state gets at T, T+just-under and
// T+exactly the keep.
func keptFor(state dforge.CIState, keep time.Duration) int {
	clock := &fakeClock{at: time.Date(2026, 11, 6, 15, 0, 0, 0, time.UTC)}
	c := newCICache(clock.now)
	prs, key, ask, asks := oneOpen(state)
	_ = c.fill(context.Background(), prs, key, ask)
	clock.at = clock.at.Add(keep - time.Second)
	_ = c.fill(context.Background(), prs, key, ask)
	clock.at = clock.at.Add(time.Second)
	_ = c.fill(context.Background(), prs, key, ask)
	return *asks
}

// #594: a repository with no CI asked every open change's statuses on every
// poll, which is what put two Bitbucket projects over the hourly limit. "None"
// is kept five minutes on its head - a push changes the head, so new CI on a
// new commit is still seen at once.
func TestCICache_KeepsNoneForFiveMinutes_issue594(t *testing.T) {
	if got := keptFor(dforge.CINone, 5*time.Minute); got != 2 {
		t.Errorf("asks = %d, want 2: once, kept until five minutes, asked again at five", got)
	}
}

// Running is kept a minute: long enough to stop asking on a poll a second
// after the last, short enough that a finished run shows within a poll.
func TestCICache_AsksRunningAgainAfterAMinute_issue594(t *testing.T) {
	if got := keptFor(dforge.CIRunning, time.Minute); got != 2 {
		t.Errorf("asks = %d, want 2: once, kept until a minute, asked again at one", got)
	}
}

// A failure is kept five minutes too: a retried job changes it, which is why
// it is not kept for good (#454's review), but not every poll.
func TestCICache_KeepsFailingForFiveMinutes_issue594(t *testing.T) {
	if got := keptFor(dforge.CIFailing, 5*time.Minute); got != 2 {
		t.Errorf("asks = %d, want 2", got)
	}
}

// A pass is a head's for good, as before.
func TestCICache_PassIsKeptForGood_issue594(t *testing.T) {
	if got := keptFor(dforge.CIPassing, 24*time.Hour); got != 1 {
		t.Errorf("asks = %d, want 1: a pass is never asked again", got)
	}
}

// A rate limit met while asking CI stops the asks still to start and fails
// the list: thirty more requests against a forge that said "later" make the
// wait longer, and a list with CI marks silently missing reads as "no CI".
func TestCIFill_RateLimitStopsAsks_issue594(t *testing.T) {
	c := newCICache(time.Now)
	prs := make([]dforge.PR, 20)
	for i := range prs {
		prs[i] = dforge.PR{Number: i + 1, State: dforge.Open, Head: "h" + strconv.Itoa(i)}
	}
	var mu sync.Mutex
	asks := 0
	ask := func(context.Context, dforge.PR) (dforge.CIState, error) {
		mu.Lock()
		defer mu.Unlock()
		asks++
		return dforge.CIPassing, &dforge.RateLimitError{Host: "bitbucket.org"}
	}
	err := c.fill(context.Background(), prs, func(pr dforge.PR) string { return pr.Head }, ask)
	var limited *dforge.RateLimitError
	if !errors.As(err, &limited) {
		t.Fatalf("fill = %v, want the RateLimitError", err)
	}
	if asks > ciParallel {
		t.Errorf("asks = %d, want at most %d: none started after the limit", asks, ciParallel)
	}
	for _, pr := range prs {
		if pr.CI != dforge.CINone {
			t.Errorf("#%d CI = %v, want none from a failed ask", pr.Number, pr.CI)
		}
	}
}

// #594's review: a verdict's time is the poll's, not the answer's. Kept from
// when a slow answer landed, a running change outlived the next tick - a
// minute after the last - and was asked every second poll, not every poll.
func TestCICache_KeepIsCountedFromThePoll_issue594(t *testing.T) {
	clock := &fakeClock{at: time.Date(2026, 11, 6, 15, 0, 0, 0, time.UTC)}
	c := newCICache(clock.now)
	prs, key, _, _ := oneOpen(dforge.CIRunning)
	asks := 0
	slow := func(context.Context, dforge.PR) (dforge.CIState, error) {
		asks++
		clock.at = clock.at.Add(5 * time.Second) // the answer's latency
		return dforge.CIRunning, nil
	}
	start := clock.at
	_ = c.fill(context.Background(), prs, key, slow)
	clock.at = start.Add(ciKeepRunning)
	_ = c.fill(context.Background(), prs, key, slow)
	if asks != 2 {
		t.Errorf("asks = %d, want 2: the next poll, a minute after the first began, asks again", asks)
	}
}
