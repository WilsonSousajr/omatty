// These tests guard the module-hygiene steps in .github/workflows/ci.yml (#261):
// the pinned Go toolchain, `go mod tidy -diff` and `govulncheck`.
package scripts_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Regression, issue #261: ci.yml asked for go-version "1.26", which
// actions/setup-go resolves to the newest patch - it was building on go1.26.8
// while go.mod said go1.26.5 and this machine ran go1.26.5. Nothing reported
// the gap, and it was not cosmetic: govulncheck found GO-2026-6088 reachable
// through internal/highlight on 1.26.5 and nothing at all on 1.26.8, so the
// same gate would have passed on CI and failed locally.
//
// The repo already pins golangci-lint for exactly this reason, and ci.yml says
// so in a comment: "Pinned so CI and the local gate run the identical linter."
// The toolchain that compiles everything deserves the same treatment.
func TestCI_PinsAnExactGoVersion(t *testing.T) {
	version := ciGoVersion(t)

	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(version) {
		t.Fatalf("ci.yml pins go-version %q, want an exact x.y.z\n"+
			"a floating minor lets CI drift onto a different toolchain than "+
			"the local gate, silently", version)
	}
	if want := modGoVersion(t); version != want {
		t.Errorf("ci.yml builds with Go %s but go.mod declares %s; "+
			"the local gate and CI must compile the same source with the "+
			"same toolchain", version, want)
	}
}

// The two hygiene steps must actually be in the workflow. They are cheap and
// easy to drop in a merge conflict, and their absence is invisible: a gate
// that stops checking simply goes on passing.
func TestCI_RunsTheHygieneSteps(t *testing.T) {
	workflow := ciWorkflow(t)

	for _, step := range []string{"go mod tidy -diff", "govulncheck ./..."} {
		if !strings.Contains(workflow, step) {
			t.Errorf("ci.yml does not run %q", step)
		}
	}
}

// govulncheck is pinned for the same reason golangci-lint is: a scanner that
// changes under you reports different things on different days, and the gate
// cannot tell a new finding from a new scanner.
func TestCI_PinsGovulncheck(t *testing.T) {
	workflow := ciWorkflow(t)

	if !strings.Contains(workflow, "GOVULN_VERSION:") {
		t.Fatal("ci.yml does not pin GOVULN_VERSION")
	}
	if strings.Contains(workflow, "golang.org/x/vuln/cmd/govulncheck@latest") {
		t.Error("govulncheck is installed at @latest; pin it like golangci-lint")
	}
}

// ciGoVersion reads the toolchain ci.yml pins, and checks setup-go is actually
// given it - a version named in the env block and then ignored at the use site
// would pass every other assertion here while pinning nothing.
func ciGoVersion(t *testing.T) string {
	t.Helper()
	workflow := ciWorkflow(t)
	if !strings.Contains(workflow, "go-version: ${{ env.GO_VERSION }}") {
		t.Fatal("actions/setup-go does not take its version from GO_VERSION")
	}
	match := regexp.MustCompile(`GO_VERSION:\s*"?([0-9.]+)"?`).FindStringSubmatch(workflow)
	if match == nil {
		t.Fatal("ci.yml sets no GO_VERSION")
	}
	return match[1]
}

// modGoVersion reads the go directive from go.mod.
func modGoVersion(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if version, found := strings.CutPrefix(line, "go "); found {
			return strings.TrimSpace(version)
		}
	}
	t.Fatal("go.mod has no go directive")
	return ""
}

// ciWorkflow reads the only CI workflow.
func ciWorkflow(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
