package forge_test

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// azureAt is a Router over url, named azure when hosts says so, with az.
func azureAt(t *testing.T, url string, hosts forge.Hosts, az string, api *FakeAzureAPI) *forge.Router {
	return forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: url}).url, Hosts: hosts},
		Bins:    map[forge.Kind]string{forge.KindAzure: az}, API: api.serve(t),
	})
}

// az's token is an Entra token for Azure DevOps Services: it is sent to
// dev.azure.com and *.visualstudio.com, never to an Azure DevOps Server the
// operator named - which does not take it anyway (#456's review).
func TestAzure_AzsTokenGoesOnlyToServices_issue456(t *testing.T) {
	az, _ := fakeAz(t, secret)
	server := forge.Hosts{"tfs.corp.example": forge.KindAzure}
	for _, url := range []string{"https://tfs.corp.example/tfs/DefaultCollection/Proj/_git/repo", "ssh://git@tfs.corp.example:22/tfs/DefaultCollection/Proj/_git/repo"} {
		api := &FakeAzureAPI{}
		_, err := azureAt(t, url, server, az, api).ListPRs(t.TempDir())
		if err == nil || len(api.Got) != 0 {
			t.Errorf("%s: %v after %+v, want nothing sent to a Server", url, err, api.Got)
		}
	}
	api := &FakeAzureAPI{}
	_, _ = azureAt(t, "https://fabrikam.visualstudio.com/DefaultCollection/Proj/_git/repo", nil, az, api).ListPRs(t.TempDir())
	if len(api.Got) == 0 || api.Got[0].Auth != "Bearer "+secret {
		t.Errorf("visualstudio.com: sent %+v, want az's token", api.Got)
	}
}

// An az that answers without a token is a login missing - "az has no login",
// not "az is not installed"; one that gives no answer in time is an outage,
// asked again next poll rather than stopping the project (#456's review).
func TestAzure_AnAzWithoutATokenSaysWhy_issue456(t *testing.T) {
	notLoggedIn, _ := fakeGH(t, "", "ERROR: Please run 'az login' to setup account.", 1)
	_, err := azureRouter(t, notLoggedIn, nil, &FakeAzureAPI{}).ListPRs(t.TempDir())
	var missing *forge.MissingToolError
	if !errors.As(err, &missing) || missing.Tool != "az" || missing.NoLoginFor == "" {
		t.Errorf("not logged in: %v, want az's missing login named", err)
	}
	slow := filepath.Join(t.TempDir(), "az")
	if werr := os.WriteFile(slow, []byte("#!/bin/sh\nsleep 5\n"), 0o700); werr != nil {
		t.Fatal(werr)
	}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: azureRemote}).url},
		Bins:    map[forge.Kind]string{forge.KindAzure: slow}, API: (&FakeAzureAPI{}).serve(t), AzWait: 200 * time.Millisecond,
	})
	if _, err := r.ListPRs(t.TempDir()); err == nil || errors.As(err, &missing) {
		t.Errorf("slow az: %v, want an outage", err)
	}
}

// A policy that does not apply - a path-filtered build, a status policy
// waiting for its status - is not CI, and never "running" (#456's review);
// a disabled one is not CI either, and a broken one failed.
func TestAzure_OnlyAPolicyThatAppliesIsCI_issue456(t *testing.T) {
	az, _ := fakeAz(t, secret)
	for want, policies := range map[forge.CIState][]string{
		forge.CINone:    {azEvaluation("Build", "notApplicable"), azEvaluation("Status", "notApplicable")},
		forge.CIPassing: {azEvaluation("Build", "approved"), strings.Replace(azEvaluation("Build", "rejected"), `"isEnabled": true`, `"isEnabled": false`, 1)},
		forge.CIFailing: {azEvaluation("Build", "broken")},
	} {
		api := &FakeAzureAPI{Policy: `{"value": [` + strings.Join(policies, ",") + `]}`}
		prs, err := azureRouter(t, az, nil, api).ListPRs(t.TempDir())
		if pr, _ := prNumbered(prs, 11); err != nil || pr.CI != want {
			t.Errorf("%v: CI = %v, %v; want %v", policies, pr.CI, err, want)
		}
	}
}

// A remote on an ssh Host alias - Microsoft's own advice for one key per
// organisation - keeps its organisation: v3 is ssh's prefix, not an org
// (#456's review, the Azure half of #576).
func TestAzure_AnSSHAliasKeepsItsOrganisation_issue456(t *testing.T) {
	az, _ := fakeAz(t, secret)
	ssh, _ := sshSays(t, "ssh.dev.azure.com")
	api := &FakeAzureAPI{}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "git@devops_fabrikam:v3/dnceng-public/public/dotnet-public-wiki"}).url},
		Bins:    map[forge.Kind]string{forge.KindAzure: az}, API: api.serve(t), SSH: ssh,
	})

	_, _ = r.ListPRs(t.TempDir())

	if len(api.Got) == 0 || api.Got[0].Host != "dev.azure.com" || !strings.HasPrefix(api.Got[0].Path, "/dnceng-public/public/_apis/") {
		t.Errorf("sent %+v, want dev.azure.com/dnceng-public/public", api.Got)
	}
}

// An ssh remote's path is written escaped (My%20Project); it is read as the
// name it is, sent escaped once, and the page b opens is escaped too
// (#456's review).
func TestAzure_AnEscapedPathIsEscapedOnce_issue456(t *testing.T) {
	az, _ := fakeAz(t, secret)
	var opened []string
	api := &FakeAzureAPI{}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "git@ssh.dev.azure.com:v3/fabrikam/My%20Project/repo"}).url},
		Bins:    map[forge.Kind]string{forge.KindAzure: az}, API: api.serve(t),
		Open: func(u string) error { opened = append(opened, u); return nil },
	})
	_, _ = r.ListPRs(t.TempDir())
	_ = r.BrowsePR(t.TempDir(), 7)

	if len(api.Got) == 0 || api.Got[0].Host != "dev.azure.com" || !strings.HasPrefix(api.Got[0].Path, "/fabrikam/My%20Project/_apis/") {
		t.Errorf("sent %+v, want /fabrikam/My%%20Project on dev.azure.com", api.Got)
	}
	if len(opened) != 1 || opened[0] != "https://dev.azure.com/fabrikam/My%20Project/_git/repo/pullrequest/7" {
		t.Errorf("opened %v, want the escaped page", opened)
	}
}

// Every call pins the API version Azure asks for, a WIQL query asks for no
// more than the window, and CI is read for the pull request's own project.
func TestAzure_EveryCallPinsItsVersion_issue456(t *testing.T) {
	az, _ := fakeAz(t, secret)
	api := &FakeAzureAPI{}
	r := azureRouter(t, az, nil, api)
	_, _ = r.ListPRs(t.TempDir())
	_, _ = r.ListIssues(t.TempDir())

	for _, got := range api.Got {
		if !strings.Contains(got.Path, "api-version=7.1") {
			t.Errorf("%s pins no api-version=7.1", got.Path)
		}
		if strings.Contains(got.Path, "/wit/wiql") && !strings.Contains(got.Path, "$top=100") {
			t.Errorf("%s asks for more than the window", got.Path)
		}
		if strings.Contains(got.Path, "/policy/evaluations") && !strings.Contains(got.Path, "CodeReviewId%2F") {
			t.Errorf("%s names no pull request artifact", got.Path)
		}
	}
}

// An abandoned pull request is closed.
func TestAzure_AnAbandonedPullRequestIsClosed_issue456(t *testing.T) {
	az, _ := fakeAz(t, secret)
	abandoned := `{"value": [{"pullRequestId": 3, "title": "t", "status": "abandoned", "sourceRefName": "refs/heads/x",
	 "lastMergeSourceCommit": {"commitId": "abc"}, "repository": {"project": {"id": "p"}}}]}`
	api := &FakeAzureAPI{First: []giteaAnswer{{match: "searchCriteria.status=abandoned", status: 200, body: abandoned}}}

	prs, err := azureRouter(t, az, nil, api).ListPRs(t.TempDir())

	if pr, ok := prNumbered(prs, 3); err != nil || !ok || pr.State != forge.Closed {
		t.Errorf("!3 = %+v, %v; want closed", pr, err)
	}
}

// A PAT the operator set is the credential for Azure DevOps: az's login may
// be another tenant's, whose refusal stopped the project with the PAT never
// tried (#457's review). An explicit variable wins, as GH_TOKEN does over
// gh's stored login - and az is not even started.
func TestAzureREST_APATIsPreferredToAzsLogin_issue457(t *testing.T) {
	az, calls := fakeAz(t, "another-tenants-token")
	api := &FakeAzureAPI{}

	_, err := azureRouter(t, az, azurePAT, api).ListPRs(t.TempDir())

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+secret))
	if err != nil || len(api.Got) == 0 || api.Got[0].Auth != want {
		t.Errorf("ListPRs = %v, sent %+v; want the PAT", err, api.Got)
	}
	if b, _ := os.ReadFile(calls); len(b) != 0 {
		t.Errorf("az was asked %q with a PAT set", b)
	}
}

// Nor does the PAT reach an Azure DevOps Server; and with the CLI forced, the
// PAT is not the CLI (#457's review).
func TestAzureREST_ThePATStaysWithServicesAndOffTheCLI_issue457(t *testing.T) {
	api := &FakeAzureAPI{}
	server := forge.Hosts{"tfs.corp.example": forge.KindAzure}
	r := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: "https://tfs.corp.example/tfs/DefaultCollection/Proj/_git/repo"}).url, Hosts: server},
		Env:     azurePAT, API: api.serve(t),
	})
	if _, err := r.ListPRs(t.TempDir()); err == nil || len(api.Got) != 0 {
		t.Errorf("a Server: %v after %+v, want nothing sent", err, api.Got)
	}
	cli := forge.NewTestRouter(forge.TestEnv{
		Options: forge.Options{Remote: (&FakeRemote{URL: azureRemote}).url, Transport: forge.TransportCLI},
		Env:     azurePAT, API: api.serve(t),
	})
	var missing *forge.MissingToolError
	if _, err := cli.ListPRs(t.TempDir()); !errors.As(err, &missing) || len(api.Got) != 0 {
		t.Errorf("CLI forced: %v after %+v, want az missing and nothing sent", err, api.Got)
	}
}

// az's token asks Azure not to redirect to its sign-in page too.
func TestAzure_AzsLoginAsksForNoSignIn_issue457(t *testing.T) {
	az, _ := fakeAz(t, secret)
	api := &FakeAzureAPI{}

	_, _ = azureRouter(t, az, nil, api).ListPRs(t.TempDir())

	if len(api.FedAuth) == 0 || api.FedAuth[0] != "Suppress" {
		t.Errorf("FedAuthRedirect = %v, want Suppress", api.FedAuth)
	}
}

// Azure's 203 and an HTML page are each a refused credential, alone.
func TestAzureREST_A203AndAnHTMLPageAreEachARefusal_issue457(t *testing.T) {
	for name, api := range map[string]*FakeAzureAPI{
		"203 with JSON's type": {Status: 203},
		"200 of HTML":          {First: []giteaAnswer{{match: "pullrequests", status: 200, contentType: "text/html", body: "<html>sign in</html>"}}},
	} {
		_, err := azureRouter(t, "", azurePAT, api).ListPRs(t.TempDir())
		var refused *forge.AuthError
		if !errors.As(err, &refused) {
			t.Errorf("%s: error = %v, want the PAT refused", name, err)
		}
	}
}
