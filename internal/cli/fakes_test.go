package cli_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
	statestore "github.com/WilsonSousajr/omatty/internal/infra/store"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
)

// FakeStore is a state.json held in memory, failing every load when Err is set.
type FakeStore struct {
	State session.State
	Err   error
}

func (f *FakeStore) Load(context.Context) (session.State, error) { return f.State, f.Err }

func (f *FakeStore) Save(context.Context, session.State) error {
	return errors.New("FakeStore: a read-only command saved")
}

// FakeGit answers the git questions adopt asks from a map of directory to
// repository root: which repository a directory is in, and which branch it is
// on. MainCheckout and RepoRoot share the map because no test here runs in a
// linked worktree, where they differ (#91, #122).
type FakeGit struct {
	Roots  map[string]string
	Branch string
}

func (f *FakeGit) RepoRoot(_ context.Context, dir string) (string, error) { return f.MainCheckout(dir) }

func (f *FakeGit) MainCheckout(dir string) (string, error) {
	if root, ok := f.Roots[dir]; ok {
		return root, nil
	}
	return "", errors.New("FakeGit: not a git repository: " + dir)
}

func (f *FakeGit) CurrentBranch(context.Context, string) (string, error) { return f.Branch, nil }

// storeIn is a real state.json in a scratch directory.
func storeIn(t *testing.T) sessions.StateStore {
	t.Helper()
	return statestore.NewStore(filepath.Join(t.TempDir(), "state.json"))
}
