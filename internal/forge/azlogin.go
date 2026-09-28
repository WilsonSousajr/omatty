package forge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// azureResource is Azure DevOps's own resource id: the audience of a token az
// issues for it.
const azureResource = "499b84ac-1321-427f-aa17-267ca6975798"

// azTimeout bounds az, a Python program that takes a second or two to start.
const azTimeout = 10 * time.Second

// pickAzure reads Azure DevOps with the operator's own az login (#456): the
// token az issues for Azure DevOps, which carries SSO and refresh the way a
// CLI should, sent as a bearer. Not `az repos` or `az devops invoke`: their
// area and resource names cannot be checked without an organisation, and one
// REST fold read either way cannot disagree with itself.
func (r *Router) pickAzure(remote Remote) (backend, error) {
	if !azureServices(remote.Host) {
		// az's token is an Entra token for Azure DevOps Services. A Server
		// neither takes it nor should see it (#456's review).
		return nil, fmt.Errorf("forge: %s is an Azure DevOps Server, which omatty does not read yet (#596): %w", remote.Host, ErrNoForge)
	}
	base, project, repo, err := azureCoordinates(remote)
	if err != nil {
		return nil, err
	}
	b := azBackend{rest: r.rest, base: base, project: project, repo: repo, host: remote.Host, ci: r.ci, open: r.open}
	var azErr error = &MissingToolError{Tool: "az"}
	if bin, ok := r.cli(KindAzure); ok {
		tok, err := azToken(bin, r.azWait)
		if err == nil {
			b.auth, b.env = noSignIn(bearer(tok)), "az's login"
			return b, nil
		}
		azErr = err
	}
	// Without az's token, the PAT az devops itself reads, as Basic auth with
	// an empty user (#457).
	if pat, env := r.borrow([]string{"AZURE_DEVOPS_EXT_PAT"}); pat != "" && r.transport != TransportCLI {
		b.auth, b.env = noSignIn(basicAuth("", pat)), env
		return b, nil
	}
	return nil, withPAT(azErr)
}

// withPAT is a missing az's note with the PAT beside it: either half fixes it.
// An az that gave no answer in time stays the outage it is.
func withPAT(err error) error {
	var missing *MissingToolError
	if !errors.As(err, &missing) {
		return err
	}
	return &MissingToolError{Tool: missing.Tool, NoLoginFor: missing.NoLoginFor, TokenEnv: "AZURE_DEVOPS_EXT_PAT"}
}

// noSignIn asks Azure to answer a refused credential with a 401 rather than a
// redirect to its interactive sign-in page, which is HTML a terminal has no
// use for (#457). An answer that is still HTML is a refusal all the same.
func noSignIn(a auth) auth {
	return func(r *http.Request) {
		a(r)
		r.Header.Set("X-TFS-FedAuthRedirect", "Suppress")
	}
}

// azureServices is whether host is Azure DevOps Services - dev.azure.com, an
// organisation's visualstudio.com, or their ssh hosts - the one audience az's
// token is for.
func azureServices(host string) bool {
	return host == "dev.azure.com" || azureSSH(host) || strings.HasSuffix(host, ".visualstudio.com")
}

// azToken is the token az issues for Azure DevOps, held for the one call and
// never kept: omatty stores no token (#453). An az that answers without one -
// not logged in, no account - is a login missing, which no poll changes; one
// that gives no answer in wait is an outage - a first run compiling, a
// refresh on a waking laptop - asked again next poll (#456's review).
func azToken(bin string, wait time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "account", "get-access-token", "--resource", azureResource, "--output", "json")
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return "", fmt.Errorf("forge: az gave no token in %v: %w", wait, ctx.Err())
	}
	var got struct {
		AccessToken string `json:"accessToken"`
	}
	if err != nil || json.Unmarshal(out, &got) != nil || got.AccessToken == "" {
		return "", &MissingToolError{Tool: "az", NoLoginFor: "Azure DevOps"}
	}
	return got.AccessToken, nil
}
