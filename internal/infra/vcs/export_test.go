package vcs

import (
	"github.com/WilsonSousajr/omatty/internal/domain/review"
	"time"
)

// ParseShortstat is the parser behind CLI.Shortstat, for the table test: the
// integration test drives git for real, but git will not print a
// deletions-only or a binary-only line on demand (#180).
func ParseShortstat(line string) (review.Shortstat, error) { return parseShortstat(line) }

// NewCLIWithBin runs bin in place of git, so a test can stand a script in and
// see how the command was invoked - the environment in particular (#472).
func NewCLIWithBin(bin string) *CLI { return &CLI{bin: bin} }

// NewCLIWithDeadline is NewCLIWithBin with every call held to limit, so a test
// can stand a git that hangs in and see it cut off without waiting 30 s (#650).
func NewCLIWithDeadline(bin string, limit time.Duration) *CLI {
	return &CLI{bin: bin, limit: limit}
}

// DeadlineFor is the deadline a real CLI gives one git invocation (#650).
func DeadlineFor(args ...string) time.Duration { return NewCLI().deadlineFor(args) }
