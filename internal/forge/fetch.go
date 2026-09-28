package forge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// fetcher reads one API path's JSON for a forge's backend (#454). A forge
// whose CLI has an `api` passthrough - glab, tea - is read through the same
// paths over either transport, so the CLI and REST answers are the same bytes
// and one fold serves both: a UI test cannot tell them apart (spec rule 4).
type fetcher interface {
	get(ctx context.Context, path string) ([]byte, error)
}

// cliAPI runs `<bin> api --hostname <host> <path>` in the project's root: the
// operator's own CLI, on their own login.
type cliAPI struct {
	bin, host, dir string
	timeout        time.Duration
}

func (c cliAPI) get(ctx context.Context, path string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.bin, "api", "--hostname", c.host, path)
	cmd.Dir = c.dir
	cmd.WaitDelay = time.Second // #356, as ghCLI.run
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("forge: %s api %s gave no answer in %v: %w", c.bin, path, c.timeout, ctx.Err())
	}
	if err != nil {
		return nil, apiFailure(c.host, c.bin, strings.TrimSpace(stderr.String()), err)
	}
	return out, nil
}

// httpStatusIn is the "(HTTP 404)" glab and tea end an API failure with.
var httpStatusIn = regexp.MustCompile(`\(HTTP (\d{3})\)`)

// apiFailure sorts a CLI's API failure by the HTTP status it reports, the way
// answerError sorts a REST answer: 404 is errNotFound for the backend to
// judge, 401 and 403 a login the forge refused, anything else carries the
// CLI's own words.
func apiFailure(host, bin, stderr string, err error) error {
	m := httpStatusIn.FindStringSubmatch(stderr)
	if m == nil {
		return fmt.Errorf("forge: %s api on %s: %s: %w", bin, host, stderr, err)
	}
	status, _ := strconv.Atoi(m[1])
	switch status {
	case 404:
		return fmt.Errorf("forge: %s: %s: %w", host, stderr, errNotFound)
	case 401, 403:
		return &AuthError{Host: host, TokenEnv: bin + "'s login", Status: status}
	}
	return fmt.Errorf("forge: %s api on %s: %s: %w", bin, host, stderr, err)
}

// repoMissing is a list's 404 as ErrNoForge: a list is the repository's, so
// its 404 is a repository this login cannot see. An item's 404 is left as it
// is - an item deleted a moment ago is not a project without a forge (#453).
func repoMissing(err error) error {
	if errors.Is(err, errNotFound) {
		return fmt.Errorf("%w: %w", err, ErrNoForge)
	}
	return err
}

// restAPI is the same paths over HTTP, under base, with a borrowed token.
type restAPI struct {
	rest restClient
	base string
	auth auth
	env  string
}

func (r restAPI) get(ctx context.Context, path string) ([]byte, error) {
	return r.rest.get(ctx, r.base+"/"+path, r.auth, r.env)
}
