package forge_test

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// FakeRemote stands in for vcs.CLI.RemoteURL: one URL for every project, or
// an error, and a count of how often it was asked.
type FakeRemote struct {
	URL   string
	Err   error
	mu    sync.Mutex
	Asked int
}

func (f *FakeRemote) url(string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Asked++
	return f.URL, f.Err
}

func (f *FakeRemote) asked() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Asked
}

// routerOn is a Router whose projects all have remote's origin and whose gh is
// bin, with hosts as the operator's table.
func routerOn(remote *FakeRemote, bin string, hosts forge.Hosts) *forge.Router {
	return forge.NewRouterWithBin(forge.Options{Remote: remote.url, Hosts: hosts}, bin)
}

// ranGh reports whether the fake gh behind calls was run at all.
func ranGh(t *testing.T, calls string) bool {
	t.Helper()
	_, err := os.Stat(calls)
	return err == nil
}

// Detect, never guess: a remote on another forge, or on no forge omatty
// knows, is ErrNoForge before gh is run - gh cannot answer for GitLab, and
// asking it is a process spent to learn nothing (#452).
func TestRouter_ANonGitHubRemoteIsErrNoForgeWithoutRunningGh_issue452(t *testing.T) {
	for _, url := range []string{"git@gitlab.com:group/project.git", "https://git.corp.example/o/r.git"} {
		bin, calls := fakeGH(t, "[]", "", 0)

		_, err := routerOn(&FakeRemote{URL: url}, bin, nil).ListPRs(t.TempDir())

		if !errors.Is(err, forge.ErrNoForge) {
			t.Errorf("%s: error = %v, want ErrNoForge", url, err)
		}
		if ranGh(t, calls) {
			t.Errorf("%s: gh was run for a project that is not on GitHub", url)
		}
	}
}

// A checkout with no origin, or one whose origin is not a URL, has no forge.
func TestRouter_NoReadableOriginIsErrNoForge_issue452(t *testing.T) {
	for _, remote := range []*FakeRemote{
		{Err: errors.New("vcs: reading the origin remote: no such remote")},
		{URL: "/srv/git/local.git"},
	} {
		bin, _ := fakeGH(t, "[]", "", 0)

		_, err := routerOn(remote, bin, nil).ListIssues(t.TempDir())

		if !errors.Is(err, forge.ErrNoForge) {
			t.Errorf("remote %+v: error = %v, want ErrNoForge", remote, err)
		}
	}
}

// A project's forge is resolved once and kept: the remote does not move under
// a running session, and every poll asking git again is a process per minute
// per project for nothing.
func TestRouter_ResolvesAProjectOnce_issue452(t *testing.T) {
	bin, _ := fakeGH(t, "[]", "", 0)
	remote := &FakeRemote{URL: "https://github.com/WilsonSousajr/omatty.git"}
	r := routerOn(remote, bin, nil)
	root := t.TempDir()

	for range 3 {
		if _, err := r.ListPRs(root); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.ListIssues(root); err != nil {
		t.Fatal(err)
	}

	if got := remote.asked(); got != 1 {
		t.Errorf("the remote was read %d times, want once", got)
	}
}

// A failure is not kept: a remote added after omatty started is read on the
// next poll.
func TestRouter_AFailedResolveIsAskedAgain_issue452(t *testing.T) {
	bin, _ := fakeGH(t, "[]", "", 0)
	remote := &FakeRemote{Err: errors.New("no such remote")}
	r := routerOn(remote, bin, nil)
	root := t.TempDir()
	_, _ = r.ListPRs(root)

	remote.Err, remote.URL = nil, "git@github.com:WilsonSousajr/omatty.git"
	if _, err := r.ListPRs(root); err != nil {
		t.Errorf("after origin was added: %v, want the list", err)
	}
}

// [forge.hosts] is how a GitHub Enterprise host reaches gh at all.
func TestRouter_AConfiguredHostReachesItsBackend_issue452(t *testing.T) {
	bin, calls := fakeGH(t, "[]", "", 0)
	remote := &FakeRemote{URL: "https://ghe.corp.example/team/app.git"}

	_, err := routerOn(remote, bin, forge.Hosts{"ghe.corp.example": forge.KindGitHub}).ListPRs(t.TempDir())

	if err != nil || !ranGh(t, calls) {
		t.Errorf("ListPRs = %v, ran gh %v; want gh asked for the configured host", err, ranGh(t, calls))
	}
}

// Label answers from what the Router already knows and never reads the remote
// itself: it is called while the window draws. Neutral until the first call
// has resolved the project, the forge's own words after.
func TestRouter_LabelNeverReadsTheRemote_issue452(t *testing.T) {
	bin, _ := fakeGH(t, "[]", "", 0)
	remote := &FakeRemote{URL: "https://github.com/WilsonSousajr/omatty.git"}
	r := routerOn(remote, bin, nil)
	root := t.TempDir()

	if got := r.Label(root); got != forge.Neutral || remote.asked() != 0 {
		t.Errorf("before a call: Label = %+v after %d remote reads, want Neutral and none", got, remote.asked())
	}
	_, _ = r.ListPRs(root)
	if got := r.Label(root); got != forge.GitHub {
		t.Errorf("after a call: Label = %+v, want GitHub's", got)
	}
}

// The UI calls in from several goroutines at once - both polls, an item read,
// a ship - so the cache is shared safely (run under -race).
func TestRouter_ConcurrentCallsShareTheCache_issue452(t *testing.T) {
	bin, _ := fakeGH(t, "[]", "", 0)
	r := routerOn(&FakeRemote{URL: "https://github.com/o/r.git"}, bin, nil)
	root := t.TempDir()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			_, _ = r.ListPRs(root)
			_ = r.Label(root)
		})
	}
	wg.Wait()
}

// A change is browsed with the same gh call as an issue: gh resolves the kind
// from the number. The pair exists for forges that number them apart.
func TestRouter_BrowsePRIsGhBrowseToo_issue452(t *testing.T) {
	bin, calls := fakeGH(t, "", "", 0)
	root := t.TempDir()

	if err := forge.NewCLIWithBin(bin).BrowsePR(root, 400); err != nil {
		t.Fatal(err)
	}

	out, err := os.ReadFile(calls)
	if err != nil || !strings.Contains(string(out), "|browse 400") {
		t.Errorf("calls = %q, %v; want browse 400", out, err)
	}
}

// sshSays is a fake ssh whose -G prints hostname for every alias, recording
// each call.
func sshSays(t *testing.T, hostname string) (bin, calls string) {
	return fakeGH(t, "user git\nhostname "+hostname+"\nport 22\n", "", 0)
}

// Regression, #576: a remote on an ssh Host alias - git@github-work:acme/app -
// is how people with two GitHub accounts clone, and gh resolves the alias
// itself. The Router names the forge from the host, so it must resolve the
// alias too, or those projects go quiet the day #452 lands.
func TestRouter_AnSSHAliasReachesItsForge_issue576(t *testing.T) {
	gh, ghCalls := fakeGH(t, "[]", "", 0)
	ssh, _ := sshSays(t, "github.com")
	r := forge.NewRouterWithSSH(forge.Options{Remote: (&FakeRemote{URL: "git@github-work:acme/app.git"}).url}, gh, ssh)
	root := t.TempDir()

	if _, err := r.ListPRs(root); err != nil || !ranGh(t, ghCalls) {
		t.Fatalf("ListPRs = %v, ran gh %v; want the alias read as github.com", err, ranGh(t, ghCalls))
	}
	if got := r.Label(root); got != forge.GitHub {
		t.Errorf("Label = %+v, want GitHub's", got)
	}
}

// An alias for a host omatty does not know is still no forge.
func TestRouter_AnAliasForAnUnknownHostIsErrNoForge_issue576(t *testing.T) {
	gh, _ := fakeGH(t, "[]", "", 0)
	ssh, _ := sshSays(t, "git.corp.example")
	r := forge.NewRouterWithSSH(forge.Options{Remote: (&FakeRemote{URL: "git@corp:team/app.git"}).url}, gh, ssh)

	if _, err := r.ListPRs(t.TempDir()); !errors.Is(err, forge.ErrNoForge) {
		t.Errorf("error = %v, want ErrNoForge", err)
	}
}

// ssh is asked only when nothing else answers, and only about an ssh remote:
// [forge.hosts] wins, an https host is a DNS name and never an alias, and a
// "host" that would read as an ssh option is never handed to ssh at all.
func TestRouter_SSHIsAskedOnlyWhenNothingElseCan_issue576(t *testing.T) {
	for _, tt := range []struct {
		url   string
		hosts forge.Hosts
	}{
		{"git@github-work:acme/app.git", forge.Hosts{"github-work": forge.KindGitHub}},
		{"https://git.corp.example/team/app.git", nil},
		{"git@-oProxyCommand=touch:o/r.git", nil},
		{"git@github.com:o/r.git", nil},
	} {
		gh, _ := fakeGH(t, "[]", "", 0)
		ssh, sshCalls := sshSays(t, "github.com")
		r := forge.NewRouterWithSSH(forge.Options{Remote: (&FakeRemote{URL: tt.url}).url, Hosts: tt.hosts}, gh, ssh)

		_, _ = r.ListPRs(t.TempDir())

		if ranGh(t, sshCalls) {
			t.Errorf("%s: ssh was asked", tt.url)
		}
	}
}
