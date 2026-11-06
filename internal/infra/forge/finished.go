package forge

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"

	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
)

// finishedEvery is how many polls an unchanged project reuses its finished
// list before reading it again: a change can be reopened and closed without
// the open set showing it, and once in ten minutes is cheap (#594).
const finishedEvery = 10

// finishedCache keeps each project's recently finished changes. They are only
// the work at a worktree's HEAD (#358), and the list can only gain a change
// when one leaves the open set, so reading it on every poll was a second
// request a minute per project for nothing: what put two Bitbucket Cloud
// projects past the hourly limit with CI (#594). One per Router.
type finishedCache struct {
	mu   sync.Mutex
	seen map[string]finishedRead
}

// finishedRead is one project's last finished list, the open set it was read
// beside, and how many polls have reused it.
type finishedRead struct {
	open     string
	finished []dforge.PR
	polls    int
}

func newFinishedCache() *finishedCache {
	return &finishedCache{seen: map[string]finishedRead{}}
}

// read is the finished list for the project key names: the one kept when open
// is the set it was read beside and fewer than finishedEvery polls have reused
// it, else fetch's answer, kept. A failed fetch fails the list, as it always
// has, and keeps nothing.
func (c *finishedCache) read(ctx context.Context, key string, open []dforge.PR, fetch func(context.Context) ([]dforge.PR, error)) ([]dforge.PR, error) {
	set := openSet(open)
	if kept, ok := c.reuse(key, set); ok {
		return kept, nil
	}
	finished, err := fetch(ctx)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.seen[key] = finishedRead{open: set, finished: finished}
	c.mu.Unlock()
	return finished, nil
}

// reuse is key's kept list while set is unchanged and it is not due a re-read,
// counting the poll.
func (c *finishedCache) reuse(key, set string) ([]dforge.PR, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	kept, ok := c.seen[key]
	if !ok || kept.open != set || kept.polls+1 >= finishedEvery {
		return nil, false
	}
	kept.polls++
	c.seen[key] = kept
	return kept.finished, true
}

// openSet names the open changes by number and head, in order, so a push or a
// change leaving reads as a different set.
func openSet(open []dforge.PR) string {
	names := make([]string, 0, len(open))
	for _, pr := range open {
		names = append(names, strconv.Itoa(pr.Number)+"@"+pr.Head)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}
