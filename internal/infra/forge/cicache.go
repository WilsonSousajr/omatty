package forge

import (
	"context"
	"errors"
	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
	"sort"
	"sync"
	"time"
)

// ciAsks bounds how many changes one poll asks CI of. GitLab, Gitea,
// Bitbucket and Azure carry no CI in their pull request lists, so each open
// change costs a call; the most recently updated come first, and the rest are
// asked on the polls after (M16).
const ciAsks = 30

// How long a verdict other than a pass is kept on its head before it is asked
// again (#594). Asking every poll put a repository with no CI at one request
// per open change per minute, and two such Bitbucket projects past the hourly
// limit. A push changes the head, and so the key, so new CI on a new commit is
// still seen at once; these bound only how late a change on the same head is.
const (
	// ciKeepRunning is short: a run that finishes should show within a poll.
	ciKeepRunning = time.Minute
	// ciKeepFailing lets a retried job turn green within minutes (#454's
	// review), without asking every poll.
	ciKeepFailing = 5 * time.Minute
	// ciKeepNone is how late CI added to a head already pushed is seen.
	ciKeepNone = 5 * time.Minute
)

// ciVerdict is a head's CI and when it stops being known; a zero until is
// kept for good, which only a pass is.
type ciVerdict struct {
	state dforge.CIState
	until time.Time
}

// ciCache keeps each change's CI verdict by its head - a pass for good, the
// rest for a while - so a poll asks only what is not known. One per Router,
// shared by every backend.
type ciCache struct {
	mu    sync.Mutex
	now   func() time.Time
	done  map[string]ciVerdict
	asked map[string]int // the poll a change was last asked on
	poll  int
}

func newCICache(now func() time.Time) *ciCache {
	return &ciCache{now: now, done: map[string]ciVerdict{}, asked: map[string]int{}}
}

// ciParallel is how many CI asks run at once. Each is a CLI process or an HTTP
// round trip; one after another, a first poll of thirty glab calls took most of
// the thirty-second budget against gitlab.com.
const ciParallel = 6

// fill sets CI on the open changes in prs. key names a change's head across
// projects; ask reads one change's verdict. A change whose ask fails keeps
// CINone - no mark - rather than failing the whole list. A rate limit is the
// exception: no ask starts after one, and fill returns it, so the list fails
// and is waited out rather than read as having no CI (#594).
func (c *ciCache) fill(ctx context.Context, prs []dforge.PR, key func(dforge.PR) string, ask func(context.Context, dforge.PR) (dforge.CIState, error)) error {
	var wg sync.WaitGroup
	var limit limitSeen
	slots := make(chan struct{}, ciParallel)
	for _, i := range c.unknown(prs, key) {
		slots <- struct{}{}
		if limit.get() != nil {
			<-slots
			break
		}
		wg.Go(func() {
			defer func() { <-slots }()
			c.askOne(ctx, &prs[i], key, ask, &limit)
		})
	}
	wg.Wait()
	return limit.get()
}

// askOne reads one change's CI into pr, keeping a rate limit in limit.
func (c *ciCache) askOne(ctx context.Context, pr *dforge.PR, key func(dforge.PR) string, ask func(context.Context, dforge.PR) (dforge.CIState, error), limit *limitSeen) {
	state, err := ask(ctx, *pr)
	var limited *dforge.RateLimitError
	switch {
	case errors.As(err, &limited):
		limit.set(err)
	case err == nil:
		pr.CI = state
		c.remember(key(*pr), state)
	}
}

// limitSeen is the first rate limit any of fill's asks met.
type limitSeen struct {
	mu  sync.Mutex
	err error
}

func (l *limitSeen) set(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err == nil {
		l.err = err
	}
}

func (l *limitSeen) get() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.err
}

// unknown fills in every verdict already known and returns the changes still
// to ask, at most ciAsks of them: the least recently asked first, and among
// those the most recently updated. A running change is never kept, so without
// the rotation the same thirty would be asked every poll and the rest never
// (#454's review).
func (c *ciCache) unknown(prs []dforge.PR, key func(dforge.PR) string) []int {
	var ask []int
	for _, i := range byRecency(prs) {
		if state, ok := c.known(key(prs[i])); ok {
			prs[i].CI = state
		} else {
			ask = append(ask, i)
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.poll++
	sort.SliceStable(ask, func(a, b int) bool { return c.asked[key(prs[ask[a]])] < c.asked[key(prs[ask[b]])] })
	ask = ask[:min(len(ask), ciAsks)]
	for _, i := range ask {
		c.asked[key(prs[i])] = c.poll
	}
	return ask
}

// known is key's verdict while it is kept; an expired one is forgotten.
func (c *ciCache) known(key string) (dforge.CIState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.done[key]
	if ok && !v.until.IsZero() && !c.now().Before(v.until) {
		delete(c.done, key)
		return dforge.CINone, false
	}
	return v.state, ok
}

// remember keeps a pass for good, which a head keeps. A failure is kept a
// while and then asked again: a flaky job retried, or a pipeline run again,
// changes the verdict on the same head (#454's review). Running and none are
// kept a while too (#594).
func (c *ciCache) remember(key string, state dforge.CIState) {
	v := ciVerdict{state: state}
	if keep := ciKeep(state); keep > 0 {
		v.until = c.now().Add(keep)
	}
	c.mu.Lock()
	c.done[key] = v
	c.mu.Unlock()
}

// ciKeep is how long state is kept; zero is for good.
func ciKeep(state dforge.CIState) time.Duration {
	switch state {
	case dforge.CIPassing:
		return 0
	case dforge.CIRunning:
		return ciKeepRunning
	case dforge.CIFailing:
		return ciKeepFailing
	}
	return ciKeepNone
}

// byRecency is the indexes of the open changes, most recently updated first.
func byRecency(prs []dforge.PR) []int {
	var open []int
	for i, pr := range prs {
		if pr.State == dforge.Open {
			open = append(open, i)
		}
	}
	sort.SliceStable(open, func(a, b int) bool { return prs[open[a]].Updated.After(prs[open[b]].Updated) })
	return open
}
