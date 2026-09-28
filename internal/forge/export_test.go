package forge

import (
	"strings"
	"time"
)

// NewRouterWithBin is NewRouter with bin in place of gh, so a test can stand a
// script in for it.
func NewRouterWithBin(o Options, bin string) *Router {
	r := NewRouter(o)
	r.bins[KindGitHub] = bin
	return r
}

// NewRouterWithSSH is NewRouterWithBin with ssh in place of the ssh binary an
// alias is resolved through (#576).
func NewRouterWithSSH(o Options, gh, ssh string) *Router {
	r := NewRouterWithBin(o, gh)
	r.sshBin = ssh
	return r
}

// NewCLIWithBin is a Router whose every project is on github.com and whose gh
// is bin: the gh backend's own tests, which predate the Router (#452), read the
// same with it in front.
func NewCLIWithBin(bin string) *Router { return NewCLIWithTimeout(bin, listTimeout) }

// NewCLIWithTimeout is NewCLIWithBin with its own bound, so a test of a
// stalled gh takes milliseconds rather than the thirty seconds a real one
// is given (#356).
func NewCLIWithTimeout(bin string, d time.Duration) *Router {
	onGitHub := func(string) (string, error) { return "https://github.com/WilsonSousajr/omatty.git", nil }
	r := NewRouterWithBin(Options{Remote: onGitHub}, bin)
	r.timeout = d
	return r
}

// FinishedFieldsInclude reports whether the finished-pull-request field set asks
// gh for name. The request is half of what makes a field reach the fold, and a
// fold test cannot see it (#332).
func FinishedFieldsInclude(name string) bool {
	for _, f := range strings.Split(finishedFields, ",") {
		if f == name {
			return true
		}
	}
	return false
}
