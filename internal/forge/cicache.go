package forge

import (
	"context"
	"sort"
	"sync"
)

// ciAsks bounds how many changes one poll asks CI of. GitLab, Gitea,
// Bitbucket and Azure carry no CI in their pull request lists, so each open
// change costs a call; the most recently updated come first, and the rest are
// asked on the polls after (M16).
const ciAsks = 30

// ciCache keeps each change's finished CI verdict by its head, so a poll asks
// only what is still running or not yet known: a verdict cannot change until a
// push moves the head. One per Router, shared by every backend.
type ciCache struct {
	mu   sync.Mutex
	done map[string]CIState
}

func newCICache() *ciCache { return &ciCache{done: map[string]CIState{}} }

// ciParallel is how many CI asks run at once. Each is a CLI process or an HTTP
// round trip; one after another, a first poll of thirty glab calls took most of
// the thirty-second budget against gitlab.com.
const ciParallel = 6

// fill sets CI on the open changes in prs. key names a change's head across
// projects; ask reads one change's verdict. A change whose ask fails keeps
// CINone - no mark - rather than failing the whole list.
func (c *ciCache) fill(ctx context.Context, prs []PR, key func(PR) string, ask func(context.Context, PR) (CIState, error)) {
	var wg sync.WaitGroup
	slots := make(chan struct{}, ciParallel)
	for _, i := range c.unknown(prs, key) {
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			if state, err := ask(ctx, prs[i]); err == nil {
				prs[i].CI = state
				c.remember(key(prs[i]), state)
			}
		})
	}
	wg.Wait()
}

// unknown fills in every verdict already known and returns the changes still
// to ask, at most ciAsks of them, the most recently updated first.
func (c *ciCache) unknown(prs []PR, key func(PR) string) []int {
	var ask []int
	for _, i := range byRecency(prs) {
		if state, ok := c.known(key(prs[i])); ok {
			prs[i].CI = state
		} else if len(ask) < ciAsks {
			ask = append(ask, i)
		}
	}
	return ask
}

func (c *ciCache) known(key string) (CIState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, ok := c.done[key]
	return state, ok
}

// remember keeps a verdict that is final: passing or failing. Running, and
// none, are asked again.
func (c *ciCache) remember(key string, state CIState) {
	if state != CIPassing && state != CIFailing {
		return
	}
	c.mu.Lock()
	c.done[key] = state
	c.mu.Unlock()
}

// byRecency is the indexes of the open changes, most recently updated first.
func byRecency(prs []PR) []int {
	var open []int
	for i, pr := range prs {
		if pr.State == Open {
			open = append(open, i)
		}
	}
	sort.SliceStable(open, func(a, b int) bool { return prs[open[a]].Updated.After(prs[open[b]].Updated) })
	return open
}
