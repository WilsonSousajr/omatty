package scripts_test

import "testing"

// networkAllowed are the packages permitted to reach the network (#453). The
// network is a capability for the reason shelling out is: a second package
// with an HTTP client is a second, unreviewed place a borrowed token could go.
// internal/infra/forge's REST fallback is the one.
var networkAllowed = []string{"infra/forge"}

// The allowlist names the packages that import net/http - no more, no fewer -
// for the erosion reason TestDepguard_ExecAllowlistMatchesReality gives.
func TestDepguard_NetworkAllowlistMatchesReality_issue453(t *testing.T) {
	assertSameSet(t, "imports net/http", realImporters(t, "net/http"), networkAllowed)
}

// .golangci.yml's network rule must agree with networkAllowed, or the list
// above guards nothing.
func TestDepguard_NetworkRuleMatchesTheAllowlist_issue453(t *testing.T) {
	assertSameSet(t, "is exempt from .golangci.yml's network rule", exemptedPackages(t, "network"), networkAllowed)
}

// assertSameSet fails once for each package on one side and not the other.
func assertSameSet(t *testing.T, what string, got map[string]bool, want []string) {
	t.Helper()
	for _, pkg := range want {
		if !got[pkg] {
			t.Errorf("internal/%s is allowed but not found: it no longer %s", pkg, what)
		}
		delete(got, pkg)
	}
	for pkg := range got {
		t.Errorf("internal/%s %s but is not in networkAllowed; the network is a "+
			"capability, so say so in .golangci.yml and AGENTS.md", pkg, what)
	}
}
