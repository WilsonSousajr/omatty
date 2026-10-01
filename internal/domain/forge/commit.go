package forge

import "strings"

// shortestSure is the fewest leading hex characters taken as a commit: what
// Bitbucket names a pull request's head by.
const shortestSure = 12

// SameCommit is whether two names are one commit: equal, or one a prefix of
// the other at least twelve characters long - Bitbucket's twelve against
// git's forty (#460's review).
//
//	forge.SameCommit("31b8ff8dad0a", "31b8ff8dad0a4c6e8f1a2b3c4d5e6f708192a3b4") // true
func SameCommit(a, b string) bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	return a == b || len(a) >= shortestSure && strings.HasPrefix(b, a)
}
