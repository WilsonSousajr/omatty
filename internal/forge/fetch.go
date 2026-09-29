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
	// send is one write with a JSON body: #331's ship actions (#464).
	send(ctx context.Context, method, path string, body []byte) ([]byte, error)
}

// cliAPI runs `<bin> api --hostname <host> <path>` in the project's root: the
// operator's own CLI, on their own login.
type cliAPI struct {
	bin, host, dir string
	timeout        time.Duration
}

func (c cliAPI) get(ctx context.Context, path string) ([]byte, error) {
	return c.run(ctx, path, nil, "api", "--hostname", c.host, path)
}

// send is glab's --method with the body on stdin, typed as JSON: glab sends
// --input as it is, and GitLab reads an untyped body as a form.
func (c cliAPI) send(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	return c.run(ctx, path, body, "api", "--hostname", c.host, "--method", method, path,
		"--input", "-", "--header", "Content-Type: application/json")
}

func (c cliAPI) run(ctx context.Context, path string, stdin []byte, args ...string) ([]byte, error) {
	out, stderr, err := runAPI(ctx, c.bin, c.dir, stdin, args)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("forge: %s api %s gave no answer in %v: %w", c.bin, path, c.timeout, ctx.Err())
	}
	if err != nil {
		return nil, apiFailure(c.host, c.bin, strings.TrimSpace(stderr), err)
	}
	return out, nil
}

// runAPI runs one CLI api call in dir, with stdin as the request's body when
// it has one: glab's and tea's shared half.
func runAPI(ctx context.Context, bin, dir string, stdin []byte, args []string) ([]byte, string, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.WaitDelay = time.Second // #356, as ghCLI.run
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	return out, stderr.String(), err
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
	return statusError(host, bin, status, stderr)
}

// statusError is a CLI's HTTP status sorted the way answerError sorts a REST
// one: 404 is errNotFound for the backend to judge, 401 and 403 a login the
// forge refused, anything else an outage carrying the CLI's own words.
func statusError(host, bin string, status int, said string) error {
	switch status {
	case 404:
		return fmt.Errorf("forge: %s: %s: %w", host, said, errNotFound)
	case 401, 403:
		return &AuthError{Host: host, TokenEnv: bin + "'s login", Status: status}
	}
	return fmt.Errorf("forge: %s api on %s answered %d: %s", bin, host, status, said)
}

// teaAPI runs `tea api --login <name> --include /<path>` in the project's
// root. tea has no --hostname: it reads through a login, which the Router
// found by the project's host. And it prints any answer and exits 0, a 404
// included, so the status line --include puts on stderr is what says how the
// request went (#458).
type teaAPI struct {
	bin, login, host, dir string
	timeout               time.Duration
}

// statusLine is the "HTTP/1.1 404 Not Found" tea's --include prints first.
var statusLine = regexp.MustCompile(`(?m)^HTTP/\S+ (\d{3})`)

func (c teaAPI) get(ctx context.Context, path string) ([]byte, error) {
	return c.run(ctx, path, nil, "api", "--login", c.login, "--include", "/"+path)
}

// send is tea's -X with the JSON body read from stdin by -d @-.
func (c teaAPI) send(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	return c.run(ctx, path, body, "api", "--login", c.login, "--include", "-X", method, "-d", "@-", "/"+path)
}

func (c teaAPI) run(ctx context.Context, path string, stdin []byte, args ...string) ([]byte, error) {
	out, stderr, err := runAPI(ctx, c.bin, c.dir, stdin, args)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("forge: tea api %s gave no answer in %v: %w", path, c.timeout, ctx.Err())
	}
	return teaVerdict(c.host, out, stderr, err)
}

// teaVerdict sorts one finished tea call by the status line it printed.
func teaVerdict(host string, out []byte, stderr string, err error) ([]byte, error) {
	m := statusLine.FindStringSubmatch(stderr)
	switch {
	case err != nil:
		return nil, fmt.Errorf("forge: tea api on %s: %s: %w", host, said(stderr), err)
	case m == nil:
		return nil, fmt.Errorf("forge: tea api on %s printed no status line: %s", host, said(stderr))
	}
	if status, _ := strconv.Atoi(m[1]); status >= 300 {
		return nil, statusError(host, "tea", status, said(string(out)))
	}
	return out, nil
}

// saidMax bounds what an error repeats of a CLI's words: a proxy's error
// page can be kilobytes, and the error reaches the log (#458's review).
const saidMax = 200

// said is a CLI's words on one line, cut at saidMax.
func said(s string) string {
	line := cleanLine(strings.TrimSpace(s))
	if r := []rune(line); len(r) > saidMax {
		return string(r[:saidMax]) + "…"
	}
	return line
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

func (r restAPI) send(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	return r.rest.send(ctx, method, r.base+"/"+path, bytes.NewReader(body), r.auth, r.env)
}
