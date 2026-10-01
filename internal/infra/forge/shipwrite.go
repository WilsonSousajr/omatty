// #331's three actions on every forge but gh's own (#464): open a change, merge
// one that is already green, and read whether a branch is protected. Each is
// one write through the forge's fetcher, CLI or REST alike, and each holds
// #331's bounds in its own body: never a merge when checks go green, never a
// branch deleted, and protection read from the forge itself, failing closed.

package forge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
)

// writeJSON encodes body, sends it, and decodes the answer into T. A write
// whose answer omatty does not read passes struct{}; an empty answer - Gitea's
// merge says nothing - is T's zero, not an error.
func writeJSON[T any](what string, body any, send func([]byte) ([]byte, error)) (T, error) {
	var out T
	raw, err := json.Marshal(body)
	if err != nil {
		return out, fmt.Errorf("forge: encoding %s: %w", what, err)
	}
	answer, err := send(raw)
	if err != nil || len(bytes.TrimSpace(answer)) == 0 {
		return out, err
	}
	if err := json.Unmarshal(answer, &out); err != nil {
		return out, fmt.Errorf("forge: reading the answer to %s: %w", what, err)
	}
	return out, nil
}

// sendJSON is writeJSON through a backend's fetcher.
func sendJSON[T any](ctx context.Context, f fetcher, method, path string, body any) (T, error) {
	return writeJSON[T](method+" "+path, body, func(b []byte) ([]byte, error) {
		return f.send(ctx, method, path, b)
	})
}

// opened is a new change's number, or an error naming the host that answered
// with none: a zero would be a change the card could never find.
func opened(n int, host, change string) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("forge: %s answered with no %s number", host, change)
	}
	return n, nil
}

// protectedFlag is a branch's own protected flag, failing closed: an error, or
// an answer without the flag, is protected beside the reason (#331).
func protectedFlag(flag *bool, err error, branch string) (bool, error) {
	if err != nil {
		return true, err
	}
	if flag == nil {
		return true, fmt.Errorf("forge: the answer about branch %q carried no protected flag", branch)
	}
	return *flag, nil
}

// branchPath is a branch name as a URL path: its slashes kept, which GitHub
// and Gitea read as part of the name, and everything else escaped - release#2
// is not release (#464's review).
func branchPath(branch string) string { return (&url.URL{Path: branch}).EscapedPath() }

// needsAdmin names what a refused restrictions read lacks: Bitbucket, Cloud
// and Data Center alike, shows branch restrictions to a repository admin
// alone, so a token that reads everything else is refused here (#464's
// review) - and the merge is refused with it, protected being unknowable.
func needsAdmin(err error, tokenEnv string) error {
	var refused *AuthError
	if !errors.As(err, &refused) {
		return err
	}
	return fmt.Errorf("forge: reading branch restrictions needs repository admin, which %s lacks: %w", tokenEnv, err)
}

// stillAt refuses a merge whose head moved since the card showed it green:
// the forge's head for number is now, not head (#599).
func stillAt(number int, now, head string) error {
	if SameCommit(now, head) {
		return nil
	}
	return fmt.Errorf("forge: #%d's head is %.12s now, not the %.12s that was green: refusing to merge what nobody verified", number, now, head)
}

// SameCommit is dforge.SameCommit: moved to internal/domain/forge in
// migration step 5.9 (#653), because the TUI asks it too.
//
//	forge.SameCommit("31b8ff8dad0a", "31b8ff8dad0a4c6e8f1a2b3c4d5e6f708192a3b4") // true
func SameCommit(a, b string) bool { return dforge.SameCommit(a, b) }

// globMatches is whether a branch-restriction pattern covers branch. `*`
// matches any run, slashes included - broader than some forges read it, which
// errs toward protected, the side #331 fails to.
func globMatches(pattern, branch string) bool {
	var re strings.Builder
	re.WriteString("^")
	for _, r := range pattern {
		switch r {
		case '*':
			re.WriteString(".*")
		case '?':
			re.WriteString(".")
		default:
			re.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	matched, err := regexp.MatchString(re.String()+"$", branch)
	return matched || err != nil
}
