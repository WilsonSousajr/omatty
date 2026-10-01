package cli_test

import (
	"context"
	"errors"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
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
