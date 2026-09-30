package gate

import (
	"context"

	"github.com/WilsonSousajr/omatty/internal/domain/coverage"
)

// ProfileReader reads the coverage profile a gate's coverage step declares,
// keyed against the module at dir (ADR 0001's port, migration step 5.3, #653).
// The TUI calls it after a gate finishes; internal/infra/fsread implements it,
// because reading a file is infra's business.
//
//	p, err := profiles.Load(ctx, filepath.Join(sess.Dir, step.Profile), sess.Dir)
type ProfileReader interface {
	Load(ctx context.Context, path, dir string) (coverage.Profile, error)
}
