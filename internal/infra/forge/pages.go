package forge

import (
	"context"
	"sync"
)

// apiPage is the most one page answers: Gitea's own default cap, and
// Bitbucket's.
const apiPage = 50

// pages reads up to most items, every page at once. Gitea and Bitbucket cap a
// page at fifty and #358's windows ask for a hundred, and one page of
// codeberg.org/forgejo/forgejo's open pull requests took ten seconds: read one
// after another, two would spend most of the call's budget. read fetches one
// page, from 1; pages past the first short one are dropped.
func pages[T any](ctx context.Context, most int, read func(context.Context, int) ([]T, error)) ([]T, error) {
	n := (most + apiPage - 1) / apiPage
	got, errs := make([][]T, n), make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { got[i], errs[i] = read(ctx, i+1) })
	}
	wg.Wait()
	return joinPages(got, errs)
}

// joinPages is the pages in order, up to and including the first short one.
func joinPages[T any](got [][]T, errs []error) ([]T, error) {
	var all []T
	for i, items := range got {
		if errs[i] != nil {
			return nil, errs[i]
		}
		all = append(all, items...)
		if len(items) < apiPage {
			break
		}
	}
	return all, nil
}
