package forge_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/forge"
)

// Every fold carries the branch a pull request would merge into, so ctrl+o p
// reads protection on that branch and not only on the session's base (#598):
// a pull request Claude opened against main read develop's protection and
// merged into main.
func TestForge_EveryFoldCarriesItsTarget_issue598(t *testing.T) {
	glab, _ := gitLabRouter(t, "git@gitlab.com:gitlab-org/cli.git", nil)
	tea, _ := teaRouter(t, codebergLogin)
	az, _ := fakeAz(t, secret)
	for _, c := range []struct {
		name   string
		r      *forge.Router
		number int
		want   string
	}{
		{"gh", forge.NewCLIWithBin(recordedGh(t)), 494, "main"},
		{"GitHub REST", httpRouter(t, "git@github.com:WilsonSousajr/omatty.git", map[string]string{"GH_TOKEN": secret}, recordedGitHub(t), nil), 577, "develop"},
		{"GitLab", glab, 3966, "feat/dependency-firewall"},
		{"Gitea", tea, 14587, "forgejo"},
		{"Bitbucket", bitbucketRouter(t, bitbucketToken, &FakeBitbucketAPI{}), 1113, "atlascode-spike-main"},
		{"Bitbucket Data Center", dcRouter(t, "https://git.corp.example/scm/ops/platform.git", dcToken, &FakeBitbucketDCAPI{}), 42, "main"},
		{"Azure DevOps", azureRouter(t, az, nil, &FakeAzureAPI{}), 11, "main"},
	} {
		prs, err := c.r.ListPRs(t.TempDir())
		if pr, ok := prNumbered(prs, c.number); err != nil || !ok || pr.Base != c.want {
			t.Errorf("%s: #%d base = %q (%v, found %v), want %q", c.name, c.number, pr.Base, err, ok, c.want)
		}
	}
}
