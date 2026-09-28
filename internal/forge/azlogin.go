package forge

import (
	"context"
	"encoding/json"
	"os/exec"
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
	base, project, repo, err := azureCoordinates(remote)
	if err != nil {
		return nil, err
	}
	b := azBackend{rest: r.rest, base: base, project: project, repo: repo, host: remote.Host, ci: r.ci, open: r.open}
	if bin, ok := r.cli(KindAzure); ok {
		if tok := azToken(bin); tok != "" {
			b.auth, b.env = bearer(tok), "az's login"
			return b, nil
		}
	}
	return nil, &MissingToolError{Tool: "az"}
}

// azToken is the token az issues for Azure DevOps, or "" when az cannot - not
// logged in, or no subscription. It is held for the one call and never kept:
// omatty stores no token (#453).
func azToken(bin string) string {
	ctx, cancel := context.WithTimeout(context.Background(), azTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "account", "get-access-token", "--resource", azureResource, "--output", "json")
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	var got struct {
		AccessToken string `json:"accessToken"`
	}
	if err != nil || json.Unmarshal(out, &got) != nil {
		return ""
	}
	return got.AccessToken
}
