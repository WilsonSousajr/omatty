// Package fsread reads the files omatty only ever reads - a coverage profile,
// the go.mod beside it - on behalf of the services that declare a port for
// them. Reading a file is infra's business (ADR 0001, migration step 5.3,
// #653); what the bytes mean is the domain's.
//
//	p, err := fsread.CoverageProfiles{}.Load(ctx, filepath.Join(dir, "cover.out"), dir)
package fsread

import (
	"context"

	"github.com/WilsonSousajr/omatty/internal/domain/coverage"
)

// CoverageProfiles is service/gate's ProfileReader over the filesystem.
type CoverageProfiles struct{}

// Load reads the profile at path, keyed against the module declared by the
// go.mod at dir. A read of one local file has no deadline worth setting, so
// ctx is accepted for the port's shape and not consulted.
//
//	p, err := fsread.CoverageProfiles{}.Load(ctx, path, sess.Dir)
func (CoverageProfiles) Load(_ context.Context, path, dir string) (coverage.Profile, error) {
	return Load(path, dir, ModulePath(dir))
}
