package forge

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"
)

// Regression, #454's review: a running change is never kept, so the same thirty
// most recently updated were asked every poll and older ones never. The
// changes least recently asked go first, so every one is reached in turn.
func TestCICache_AsksTheLeastRecentlyAskedFirst_issue454(t *testing.T) {
	c := newCICache()
	prs := make([]PR, 40)
	for i := range prs {
		prs[i] = PR{Number: i + 1, State: Open, Head: "h" + strconv.Itoa(i), Updated: time.Unix(int64(1000-i), 0)}
	}
	key := func(pr PR) string { return pr.Head }
	var mu sync.Mutex
	var asked []int
	ask := func(_ context.Context, pr PR) (CIState, error) {
		mu.Lock()
		defer mu.Unlock()
		asked = append(asked, pr.Number)
		return CIRunning, nil
	}

	c.fill(context.Background(), prs, key, ask)
	first := len(asked)
	asked = nil
	c.fill(context.Background(), prs, key, ask)

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
